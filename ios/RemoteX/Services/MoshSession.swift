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

    // Serializes access to inputWriteFD, isConnected, pendingResize, and winSizePtr writes
    private let sessionQueue = DispatchQueue(label: "com.remotex.moshsession")

    // Pipes
    private var inputWriteFD: Int32 = -1
    private var outputReadHandle: FileHandle?

    // Resize: heap-allocated winsize so mosh_main holds a stable pointer
    private var winSizePtr: UnsafeMutablePointer<winsize>
    // Thread running mosh_main (needed for SIGWINCH); protected by threadLock
    private let threadLock = NSLock()
    private var _moshPThread: pthread_t?
    private var moshPThread: pthread_t? {
        get { threadLock.withLock { _moshPThread } }
        set { threadLock.withLock { _moshPThread = newValue } }
    }
    // Resize requested before mosh thread started; applied once thread is ready.
    // Access only under sessionQueue.
    private var pendingResize: (cols: Int, rows: Int)?

    // App lifecycle observer
    private var foregroundObserver: NSObjectProtocol?

    init() {
        winSizePtr = UnsafeMutablePointer<winsize>.allocate(capacity: 1)
        winSizePtr.initialize(to: winsize())
    }

    deinit {
        winSizePtr.deallocate()
        if let obs = foregroundObserver { NotificationCenter.default.removeObserver(obs) }
    }

    func connect(info: ConnectInfo, cols: Int, rows: Int) async throws {
        let alreadyConnected = sessionQueue.sync { isConnected }
        guard !alreadyConnected else { throw SessionError.alreadyConnected }

        // Set initial terminal size
        sessionQueue.sync {
            winSizePtr.pointee.ws_col = UInt16(cols)
            winSizePtr.pointee.ws_row = UInt16(rows)
            winSizePtr.pointee.ws_xpixel = 0
            winSizePtr.pointee.ws_ypixel = 0
        }

        // Input pipe: Swift → mosh (fds[0] = read end, fds[1] = write end)
        var inputFDs: [Int32] = [0, 0]
        pipe(&inputFDs)
        let inputReadFD  = inputFDs[0]
        let inputWriteFD = inputFDs[1]

        // Output pipe: mosh → Swift
        var outputFDs: [Int32] = [0, 0]
        pipe(&outputFDs)
        let outputReadFD  = outputFDs[0]
        let outputWriteFD = outputFDs[1]

        let fIn  = fdopen(inputReadFD,  "r")
        let fOut = fdopen(outputWriteFD, "w")
        setvbuf(fOut, nil, Int32(_IONBF), 0)

        let ip    = info.host
        let port  = String(info.port)
        let key   = info.key
        let wsPtr = winSizePtr

        sessionQueue.sync {
            self.inputWriteFD = inputWriteFD
            self.isConnected  = true
        }

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

                // Apply any resize that arrived before the thread was ready
                let pending: (cols: Int, rows: Int)? = self?.sessionQueue.sync {
                    let p = self?.pendingResize
                    self?.pendingResize = nil
                    return p
                }
                if let pending = pending {
                    self?.resize(cols: pending.cols, rows: pending.rows)
                }

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

                // Natural exit: clean up and fire didDisconnect only if not already disconnected
                var shouldNotify = false
                self?.sessionQueue.sync {
                    if self?.isConnected == true {
                        self?.isConnected  = false
                        self?.moshPThread  = nil
                        shouldNotify = true
                    }
                }
                if shouldNotify {
                    DispatchQueue.main.async {
                        self?.outputHandler?.didDisconnect(error: nil)
                    }
                }
            }
        }
    }

    func send(_ data: Data) {
        sessionQueue.sync {
            guard inputWriteFD >= 0 else { return }
            data.withUnsafeBytes { ptr in
                _ = write(inputWriteFD, ptr.baseAddress!, ptr.count)
            }
        }
    }

    func resize(cols: Int, rows: Int) {
        // Capture thread id before entering the queue so pthread_kill is not called under the lock
        let tid = moshPThread
        sessionQueue.sync {
            winSizePtr.pointee.ws_col = UInt16(cols)
            winSizePtr.pointee.ws_row = UInt16(rows)
            if tid == nil {
                pendingResize = (cols, rows)
            }
        }
        if let tid = tid {
            pthread_kill(tid, SIGWINCH)
        }
    }

    func disconnect() {
        var shouldNotify = false
        sessionQueue.sync {
            guard isConnected else { return }
            isConnected = false
            shouldNotify = true
            if inputWriteFD >= 0 {
                close(inputWriteFD)
                inputWriteFD = -1
            }
        }
        outputReadHandle?.readabilityHandler = nil
        if shouldNotify {
            outputHandler?.didDisconnect(error: nil)
        }
    }

    func observeAppLifecycle(sizeProvider: @escaping () -> (cols: Int, rows: Int)?) {
        foregroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.willEnterForegroundNotification,
            object: nil, queue: .main
        ) { [weak self] _ in
            guard let self, self.outputReadHandle != nil else { return }
            if let size = sizeProvider() {
                self.resize(cols: size.cols, rows: size.rows)
            }
        }
    }
}
