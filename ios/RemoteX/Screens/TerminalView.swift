import SwiftUI
import SwiftTerm
import MetalKit

struct TerminalView: View {
    let client: DaemonClient
    let session: Session
    @Environment(\.dismiss) private var dismiss
    @State private var connectInfo: ConnectInfo?
    @State private var connectError: String?
    @State private var isConnecting = true
    var body: some View {
        ZStack(alignment: .topLeading) {
            Color.black.ignoresSafeArea()

            if isConnecting {
                VStack(spacing: 16) {
                    ProgressView().tint(.white)
                    Text("Connecting to \(session.name)...")
                        .foregroundStyle(.secondary)
                }
            } else if let connectError {
                VStack(spacing: 16) {
                    Image(systemName: "xmark.circle")
                        .font(.largeTitle).foregroundStyle(.red)
                    Text(connectError)
                        .multilineTextAlignment(.center)
                        .foregroundStyle(.white)
                    Button("Dismiss") { dismiss() }
                        .buttonStyle(.bordered)
                }
                .padding()
            } else if let info = connectInfo {
                MoshTerminalView(connectInfo: info)
                    .ignoresSafeArea(edges: [.top, .leading, .trailing])
            }

            // Back button — always visible in top-left
            Button {
                dismiss()
            } label: {
                HStack(spacing: 4) {
                    Image(systemName: "chevron.left")
                        .font(.system(size: 14, weight: .semibold))
                    Text(session.name)
                        .font(.system(size: 14, weight: .semibold))
                }
                .foregroundStyle(.white)
                .padding(.horizontal, 12)
                .padding(.vertical, 7)
                .background(.black.opacity(0.5))
                .clipShape(Capsule())
            }
            .padding(.top, 56)
            .padding(.leading, 16)
        }
        .ignoresSafeArea(edges: [.top, .leading, .trailing])
        .persistentSystemOverlays(.hidden)
        .navigationBarHidden(true)
        .task { await connect() }
    }

    private func connect() async {
        do {
            connectInfo = try await client.requestConnect(sessionName: session.name)
            isConnecting = false
        } catch {
            connectError = "Failed to connect: \(error.localizedDescription)"
            isConnecting = false
        }
    }
}

struct MoshTerminalView: UIViewRepresentable {
    let connectInfo: ConnectInfo
    @Environment(\.keychain) private var keychain

    func makeUIView(context: Context) -> TerminalViewWithMosh {
        let creds = try? keychain.load()
        return TerminalViewWithMosh(connectInfo: connectInfo, sshKey: creds?.sshPrivateKey ?? "")
    }

    func updateUIView(_ uiView: TerminalViewWithMosh, context: Context) {}
}

final class TerminalViewWithMosh: UIView {
    private let terminalView: SwiftTerm.TerminalView
    private let moshSession = MoshSession()
    private let connectInfo: ConnectInfo
    private let sshKey: String
    private var hasConnected = false

    // UITextInputTraits — configure keyboard; self is first responder, not terminalView
    var autocorrectionType: UITextAutocorrectionType = .no
    var autocapitalizationType: UITextAutocapitalizationType = .none
    var spellCheckingType: UITextSpellCheckingType = .no
    var smartQuotesType: UITextSmartQuotesType = .no
    var smartDashesType: UITextSmartDashesType = .no
    var keyboardType: UIKeyboardType = .asciiCapable
    var returnKeyType: UIReturnKeyType = .default

    override var canBecomeFirstResponder: Bool { true }

    init(connectInfo: ConnectInfo, sshKey: String) {
        self.connectInfo = connectInfo
        self.sshKey = sshKey
        terminalView = SwiftTerm.TerminalView(frame: .zero)
        super.init(frame: .zero)

        addSubview(terminalView)
        terminalView.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            terminalView.topAnchor.constraint(equalTo: topAnchor),
            terminalView.bottomAnchor.constraint(equalTo: bottomAnchor),
            terminalView.leadingAnchor.constraint(equalTo: leadingAnchor),
            terminalView.trailingAnchor.constraint(equalTo: trailingAnchor),
        ])

        terminalView.terminalDelegate = self
        moshSession.outputHandler = self
        moshSession.observeAppLifecycle()
    }

    required init?(coder: NSCoder) { fatalError() }

    deinit {
        moshSession.disconnect()
    }

    // self (not terminalView) is the first responder so UIKeyInput handles input,
    // bypassing UITextInput's buffer management which corrupts backspace.
    override func didMoveToWindow() {
        super.didMoveToWindow()
        if window != nil {
            becomeFirstResponder()
        }
    }
}

// MARK: - UIKeyInput

extension TerminalViewWithMosh: UIKeyInput {
    var hasText: Bool { true }  // always true; prevents edge-case keyboard dismissal

    func insertText(_ text: String) {
        // iOS keyboard sends "\n" (LF 0x0a) for the Return key.
        // Terminals expect CR (0x0d). Map it, matching SwiftTerm's returnByteSequence.
        if text == "\n" {
            moshSession.send(Data([0x0d]))
        } else {
            moshSession.send(Data(text.utf8))
        }
    }

    func deleteBackward() {
        moshSession.send(Data([0x7f]))  // DEL — standard backspace for modern terminals
    }
}

// MARK: - TerminalOutputHandler

extension TerminalViewWithMosh: TerminalOutputHandler {
    func didReceiveOutput(_ data: Data) {
        let bytes = [UInt8](data)
        DispatchQueue.main.async {
            // Use terminalView.feed() — not getTerminal().feed() — so SwiftTerm
            // processes the data AND triggers an immediate UI redraw.
            self.terminalView.feed(byteArray: bytes[...])
        }
    }

    func didDisconnect(error: Error?) {}
}

// MARK: - TerminalViewDelegate

extension TerminalViewWithMosh: TerminalViewDelegate {
    func send(source: SwiftTerm.TerminalView, data: ArraySlice<UInt8>) {
        moshSession.send(Data(data))
    }

    func sizeChanged(source: SwiftTerm.TerminalView, newCols: Int, newRows: Int) {
        if !hasConnected {
            hasConnected = true
            Task {
                try? await moshSession.connect(
                    info: connectInfo,
                    sshPrivateKey: sshKey,
                    cols: newCols,
                    rows: newRows
                )
            }
        } else {
            moshSession.resize(cols: newCols, rows: newRows)
        }
    }

    func setTerminalTitle(source: SwiftTerm.TerminalView, title: String) {}

    func hostCurrentDirectoryUpdate(source: SwiftTerm.TerminalView, directory: String?) {}

    func scrolled(source: SwiftTerm.TerminalView, position: Double) {
        metalRedraw(source)
    }

    func requestOpenLink(source: SwiftTerm.TerminalView, link: String, params: [String: String]) {}

    func bell(source: SwiftTerm.TerminalView) {}

    func clipboardCopy(source: SwiftTerm.TerminalView, content: Data) {}

    func iTermContent(source: SwiftTerm.TerminalView, content: ArraySlice<UInt8>) {}

    func rangeChanged(source: SwiftTerm.TerminalView, startY: Int, endY: Int) {
        metalRedraw(source)
    }

    // SwiftTerm's scrolled() moves contentOffset but never queues a Metal redraw,
    // so the old texture is shown at the new scroll position (phantom text).
    // We reach into the view hierarchy to trigger the MTKView directly.
    private func metalRedraw(_ view: SwiftTerm.TerminalView) {
        guard let mtkView = view.subviews.first(where: { $0 is MTKView }) as? MTKView else { return }
        mtkView.setNeedsDisplay(mtkView.bounds)
    }
}
