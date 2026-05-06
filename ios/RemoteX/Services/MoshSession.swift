import Foundation
import UIKit
import Darwin
import mosh

protocol TerminalOutputHandler: AnyObject {
    func didReceiveOutput(_ data: Data)
    func didDisconnect(error: Error?)
}

final class MoshSession {
    enum SessionError: Swift.Error {
        case connectionFailed(String)
        case alreadyConnected
    }

    weak var outputHandler: TerminalOutputHandler?
    private(set) var isConnected = false

    // Pipes
    private var inputWriteFD: Int32 = -1    // Swift writes user input here
    private var outputReadHandle: FileHandle?

    // Resize: heap-allocated winsize so mosh_main holds a stable pointer
    private var winSizePtr: UnsafeMutablePointer<winsize>
    // Thread running mosh_main (needed for SIGWINCH)
    private var moshPThread: pthread_t?

    // App lifecycle observers
    private var backgroundObserver: NSObjectProtocol?
    private var foregroundObserver: NSObjectProtocol?

    init() {
        winSizePtr = UnsafeMutablePointer<winsize>.allocate(capacity: 1)
        winSizePtr.initialize(to: winsize())
    }

    deinit {
        winSizePtr.deallocate()
        if let obs = backgroundObserver { NotificationCenter.default.removeObserver(obs) }
        if let obs = foregroundObserver { NotificationCenter.default.removeObserver(obs) }
    }

    func connect(info: ConnectInfo, sshPrivateKey: String,
                 cols: Int, rows: Int) async throws {
        guard !isConnected else { throw SessionError.alreadyConnected }

        // Set initial terminal size
        winSizePtr.pointee.ws_col = UInt16(cols)
        winSizePtr.pointee.ws_row = UInt16(rows)
        winSizePtr.pointee.ws_xpixel = 0
        winSizePtr.pointee.ws_ypixel = 0

        // Input pipe: Swift → mosh (fds[0] = read end, fds[1] = write end)
        var inputFDs: [Int32] = [0, 0]
        pipe(&inputFDs)
        let inputReadFD  = inputFDs[0]
        let inputWriteFD = inputFDs[1]
        self.inputWriteFD = inputWriteFD

        // Output pipe: mosh → Swift
        var outputFDs: [Int32] = [0, 0]
        pipe(&outputFDs)
        let outputReadFD  = outputFDs[0]
        let outputWriteFD = outputFDs[1]

        let fIn  = fdopen(inputReadFD,  "r")
        let fOut = fdopen(outputWriteFD, "w")
        // Disable output buffering so terminal data arrives promptly
        setvbuf(fOut, nil, Int32(_IONBF), 0)

        // Capture params for thread closure
        let ip   = info.host
        let port = String(info.port)
        let key  = info.key
        let wsPtr = winSizePtr

        isConnected = true

        // Read output pipe and forward to terminal handler
        let readHandle = FileHandle(fileDescriptor: outputReadFD, closeOnDealloc: true)
        self.outputReadHandle = readHandle
        readHandle.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            self?.outputHandler?.didReceiveOutput(data)
        }

        // Run mosh_main on a background thread, capture pthread_t for SIGWINCH
        var tid: pthread_t?

        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            DispatchQueue.global(qos: .userInitiated).async { [weak self] in
                tid = pthread_self()
                self?.moshPThread = tid
                continuation.resume()

                mosh_main(
                    fIn, fOut, wsPtr,
                    nil, nil,       // state_callback, context (no resume in v1)
                    ip, port, key,
                    "never",        // prediction off — tmux confuses mosh's cursor tracking
                    nil, 0,         // encoded_state_buffer (no resume)
                    "0"             // predict_overwrite
                )

                fclose(fIn)
                fclose(fOut)
                self?.isConnected = false
                self?.moshPThread = nil
                DispatchQueue.main.async {
                    self?.outputHandler?.didDisconnect(error: nil)
                }
            }
        }
    }

    func send(_ data: Data) {
        guard inputWriteFD >= 0 else { return }
        data.withUnsafeBytes { ptr in
            _ = write(inputWriteFD, ptr.baseAddress!, ptr.count)
        }
    }

    func resize(cols: Int, rows: Int) {
        winSizePtr.pointee.ws_col = UInt16(cols)
        winSizePtr.pointee.ws_row = UInt16(rows)
        // Signal mosh to re-read winsize
        if let tid = moshPThread {
            pthread_kill(tid, SIGWINCH)
        }
    }

    func disconnect() {
        if inputWriteFD >= 0 {
            close(inputWriteFD)
            inputWriteFD = -1
        }
        outputReadHandle?.readabilityHandler = nil
        isConnected = false
        outputHandler?.didDisconnect(error: nil)
    }

    func observeAppLifecycle() {
        backgroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.didEnterBackgroundNotification,
            object: nil, queue: .main
        ) { [weak self] _ in
            // mosh server maintains state; UDP keepalives handle reconnection
            _ = self
        }

        foregroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.willEnterForegroundNotification,
            object: nil, queue: .main
        ) { [weak self] _ in
            // Trigger resize to refresh terminal after resume
            guard let self, let ws = self.outputReadHandle else { return }
            let size = TerminalSizeHelper.size(for: UIScreen.main.bounds)
            self.resize(cols: size.cols, rows: size.rows)
        }
    }
}
