import SwiftUI
import SwiftTerm

struct TerminalView: View {
    let client: DaemonClient
    let session: Session
    @Environment(\.dismiss) private var dismiss
    @State private var connectInfo: ConnectInfo?
    @State private var connectError: String?
    @State private var isConnecting = true
    @State private var showControls = false

    var body: some View {
        ZStack(alignment: .topTrailing) {
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

            // Floating controls — tap anywhere on terminal to toggle
            if showControls || connectError != nil {
                Button {
                    dismiss()
                } label: {
                    Label("Exit session", systemImage: "xmark.circle.fill")
                        .font(.callout.bold())
                        .foregroundStyle(.white)
                        .padding(.horizontal, 14)
                        .padding(.vertical, 8)
                        .background(.black.opacity(0.6))
                        .clipShape(Capsule())
                }
                .padding(.top, 56)
                .padding(.trailing, 16)
                .transition(.opacity.combined(with: .scale(scale: 0.9)))
            }
        }
        .ignoresSafeArea()
        .persistentSystemOverlays(.hidden)
        .navigationBarHidden(true)
        .onTapGesture {
            withAnimation(.easeInOut(duration: 0.2)) {
                showControls.toggle()
            }
        }
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

    override func layoutSubviews() {
        super.layoutSubviews()
        guard bounds.width > 0, bounds.height > 0 else { return }
        let size = TerminalSizeHelper.size(for: bounds)
        if !hasConnected {
            hasConnected = true
            Task {
                try? await moshSession.connect(
                    info: connectInfo,
                    sshPrivateKey: sshKey,
                    cols: size.cols,
                    rows: size.rows
                )
            }
        } else {
            moshSession.resize(cols: size.cols, rows: size.rows)
        }
    }
}

// MARK: - TerminalOutputHandler

extension TerminalViewWithMosh: TerminalOutputHandler {
    func didReceiveOutput(_ data: Data) {
        let bytes = [UInt8](data)
        DispatchQueue.main.async {
            self.terminalView.getTerminal().feed(byteArray: bytes)
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
        moshSession.resize(cols: newCols, rows: newRows)
    }

    func setTerminalTitle(source: SwiftTerm.TerminalView, title: String) {}

    func hostCurrentDirectoryUpdate(source: SwiftTerm.TerminalView, directory: String?) {}

    func scrolled(source: SwiftTerm.TerminalView, position: Double) {}

    func requestOpenLink(source: SwiftTerm.TerminalView, link: String, params: [String: String]) {}

    func bell(source: SwiftTerm.TerminalView) {}

    func clipboardCopy(source: SwiftTerm.TerminalView, content: Data) {}

    func iTermContent(source: SwiftTerm.TerminalView, content: ArraySlice<UInt8>) {}

    func rangeChanged(source: SwiftTerm.TerminalView, startY: Int, endY: Int) {}
}
