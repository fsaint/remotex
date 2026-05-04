import Foundation
import UIKit

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
    private var backgroundObserver: NSObjectProtocol?
    private var foregroundObserver: NSObjectProtocol?

    // NOTE: Replace connect() body with real libmosh calls when xcframework is available.
    // Params map to: MoshParams(host:port:key:sshKey:cols:rows:)
    func connect(info: ConnectInfo, sshPrivateKey: String,
                 cols: Int, rows: Int) async throws {
        guard !isConnected else { throw SessionError.alreadyConnected }
        // TODO: call libmosh connect here
        isConnected = true
    }

    func send(_ data: Data) {
        // TODO: call libmosh writeInput(data)
    }

    func resize(cols: Int, rows: Int) {
        // TODO: call libmosh resize(cols:rows:)
    }

    func disconnect() {
        // TODO: call libmosh disconnect()
        isConnected = false
        outputHandler?.didDisconnect(error: nil)
    }

    func observeAppLifecycle() {
        backgroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.didEnterBackgroundNotification,
            object: nil, queue: .main
        ) { [weak self] _ in
            // mosh-server maintains state; keepalive handles reconnection
            _ = self
        }

        foregroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.willEnterForegroundNotification,
            object: nil, queue: .main
        ) { [weak self] _ in
            // Trigger resize to refresh terminal display on resume
            _ = self
        }
    }

    deinit {
        if let obs = backgroundObserver { NotificationCenter.default.removeObserver(obs) }
        if let obs = foregroundObserver { NotificationCenter.default.removeObserver(obs) }
    }
}
