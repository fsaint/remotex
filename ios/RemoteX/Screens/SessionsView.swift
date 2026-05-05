import SwiftUI

struct SessionsView: View {
    let client: DaemonClient
    let router: AppRouter
    @State private var sessions: [Session] = []
    @State private var isLoading = false
    @State private var error: String?
    @State private var selectedSession: Session?

    var body: some View {
        NavigationStack {
            Group {
                if isLoading && sessions.isEmpty {
                    ProgressView("Loading sessions...")
                } else if let error, sessions.isEmpty {
                    VStack(spacing: 16) {
                        Image(systemName: "exclamationmark.triangle")
                            .font(.largeTitle)
                            .foregroundStyle(.orange)
                        Text(error)
                            .multilineTextAlignment(.center)
                            .foregroundStyle(.secondary)
                        Button("Retry") { Task { await loadSessions() } }
                            .buttonStyle(.bordered)
                        Button("Re-pair with Mac") { router.unpair() }
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    }
                    .padding()
                } else {
                    List(sessions) { session in
                        SessionRow(session: session)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                if session.isLive {
                                    selectedSession = session
                                }
                            }
                    }
                    .refreshable { await loadSessions() }
                }
            }
            .navigationTitle("Sessions")
            .navigationDestination(item: $selectedSession) { session in
                TerminalView(client: client, session: session)
            }
        }
        .task { await loadSessions() }
    }

    private func loadSessions() async {
        isLoading = true
        error = nil
        do {
            sessions = try await client.listSessions()
        } catch {
            self.error = "Cannot reach Mac: \(error.localizedDescription)"
        }
        isLoading = false
    }
}

struct SessionRow: View {
    let session: Session

    var body: some View {
        HStack {
            Circle()
                .fill(session.isLive ? Color.green : Color.gray)
                .frame(width: 10, height: 10)
            VStack(alignment: .leading, spacing: 2) {
                Text(session.name)
                    .font(.headline)
                    .foregroundStyle(session.isLive ? .primary : .secondary)
                Text(session.startedAt, style: .relative)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if session.isLive {
                Image(systemName: "chevron.right")
                    .foregroundStyle(.tertiary)
            }
        }
        .padding(.vertical, 4)
    }
}
