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
    private var bottomConstraint: NSLayoutConstraint!

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
        terminalView.caretColor = .systemGreen
        terminalView.caretTextColor = .black
        terminalView.nativeBackgroundColor = .black
        super.init(frame: .zero)

        addSubview(terminalView)
        terminalView.translatesAutoresizingMaskIntoConstraints = false
        bottomConstraint = terminalView.bottomAnchor.constraint(equalTo: bottomAnchor)
        NSLayoutConstraint.activate([
            terminalView.topAnchor.constraint(equalTo: topAnchor),
            bottomConstraint,
            terminalView.leadingAnchor.constraint(equalTo: leadingAnchor),
            terminalView.trailingAnchor.constraint(equalTo: trailingAnchor),
        ])

        terminalView.terminalDelegate = self
        moshSession.outputHandler = self
        moshSession.observeAppLifecycle()

        NotificationCenter.default.addObserver(
            self,
            selector: #selector(keyboardWillChangeFrame(_:)),
            name: UIResponder.keyboardWillChangeFrameNotification,
            object: nil
        )
    }

    required init?(coder: NSCoder) { fatalError() }

    // MARK: - Keyboard toolbar

    private var ctrlPending = false
    private weak var ctrlButton: UIButton?

    private lazy var keyboardToolbar: UIView = {
        struct Key { let label: String; let sel: Selector }
        let keys: [Key] = [
            Key(label: "ESC",  sel: #selector(sendEsc)),
            Key(label: "TAB",  sel: #selector(sendTab)),
            Key(label: "CTRL", sel: #selector(toggleCtrl)),
            Key(label: "←",    sel: #selector(sendLeft)),
            Key(label: "↑",    sel: #selector(sendUp)),
            Key(label: "↓",    sel: #selector(sendDown)),
            Key(label: "→",    sel: #selector(sendRight)),
        ]
        let stack = UIStackView()
        stack.axis = .horizontal
        stack.distribution = .fillEqually
        stack.spacing = 1
        for key in keys {
            let btn = UIButton(type: .system)
            btn.setTitle(key.label, for: .normal)
            btn.titleLabel?.font = .monospacedSystemFont(ofSize: 13, weight: .medium)
            btn.setTitleColor(.white, for: .normal)
            btn.backgroundColor = UIColor(white: 0.22, alpha: 1)
            btn.layer.cornerRadius = 5
            btn.addTarget(self, action: key.sel, for: .touchUpInside)
            if key.label == "CTRL" { ctrlButton = btn }
            stack.addArrangedSubview(btn)
        }
        let bar = UIView()
        bar.backgroundColor = UIColor(white: 0.12, alpha: 1)
        bar.addSubview(stack)
        stack.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: bar.topAnchor, constant: 5),
            stack.bottomAnchor.constraint(equalTo: bar.bottomAnchor, constant: -5),
            stack.leadingAnchor.constraint(equalTo: bar.leadingAnchor, constant: 8),
            stack.trailingAnchor.constraint(equalTo: bar.trailingAnchor, constant: -8),
            bar.heightAnchor.constraint(equalToConstant: 44),
        ])
        return bar
    }()

    override var inputAccessoryView: UIView? { keyboardToolbar }

    // Hardware keyboard arrow key support
    override var keyCommands: [UIKeyCommand]? {[
        UIKeyCommand(input: UIKeyCommand.inputUpArrow,    modifierFlags: [], action: #selector(sendUp)),
        UIKeyCommand(input: UIKeyCommand.inputDownArrow,  modifierFlags: [], action: #selector(sendDown)),
        UIKeyCommand(input: UIKeyCommand.inputLeftArrow,  modifierFlags: [], action: #selector(sendLeft)),
        UIKeyCommand(input: UIKeyCommand.inputRightArrow, modifierFlags: [], action: #selector(sendRight)),
    ]}

    @objc private func sendEsc()   { moshSession.send(Data([0x1b])) }
    @objc private func sendTab()   { moshSession.send(Data([0x09])) }
    @objc private func sendLeft()  { moshSession.send(Data([0x1b, 0x5b, 0x44])) }
    @objc private func sendUp()    { moshSession.send(Data([0x1b, 0x5b, 0x41])) }
    @objc private func sendDown()  { moshSession.send(Data([0x1b, 0x5b, 0x42])) }
    @objc private func sendRight() { moshSession.send(Data([0x1b, 0x5b, 0x43])) }

    @objc private func toggleCtrl() {
        ctrlPending = !ctrlPending
        ctrlButton?.backgroundColor = ctrlPending
            ? UIColor.systemYellow.withAlphaComponent(0.7)
            : UIColor(white: 0.22, alpha: 1)
    }

    deinit {
        NotificationCenter.default.removeObserver(self)
        moshSession.disconnect()
    }

    // self (not terminalView) is the first responder so UIKeyInput handles input,
    // bypassing UITextInput's buffer management which corrupts backspace.
    override func didMoveToWindow() {
        super.didMoveToWindow()
        if window != nil {
            try? terminalView.setUseMetal(true)
            terminalView.metalBufferingMode = .perFrameAggregated
            becomeFirstResponder()
        }
    }

    @objc private func keyboardWillChangeFrame(_ notification: Notification) {
        guard
            let userInfo = notification.userInfo,
            let endFrame = userInfo[UIResponder.keyboardFrameEndUserInfoKey] as? CGRect,
            let window = self.window
        else { return }

        let localFrame = convert(endFrame, from: window)
        let overlap = max(0, bounds.maxY - localFrame.minY)

        let duration = (userInfo[UIResponder.keyboardAnimationDurationUserInfoKey] as? Double) ?? 0.25
        let curveRaw = (userInfo[UIResponder.keyboardAnimationCurveUserInfoKey] as? UInt) ?? 0
        let options = UIView.AnimationOptions(rawValue: curveRaw << 16)

        bottomConstraint.constant = -overlap
        UIView.animate(withDuration: duration, delay: 0, options: options) {
            self.layoutIfNeeded()
        }
    }
}

// MARK: - UIKeyInput

extension TerminalViewWithMosh: UIKeyInput {
    var hasText: Bool { true }  // always true; prevents edge-case keyboard dismissal

    func insertText(_ text: String) {
        // If Ctrl is active, convert the next character to a control code (e.g. C→^C 0x03).
        if ctrlPending {
            ctrlPending = false
            ctrlButton?.backgroundColor = UIColor(white: 0.22, alpha: 1)
            if let scalar = text.unicodeScalars.first {
                let v = scalar.value
                // Map @A-Z[\]^_ (64-95) and a-z (97-122) to control codes 0x00-0x1f
                if (v >= 64 && v <= 95) || (v >= 97 && v <= 122) {
                    moshSession.send(Data([UInt8(v & 0x1f)]))
                }
            }
            return
        }
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
