import SwiftUI

// Temporary stub — replaced in Task 10
struct TerminalView: View {
    let client: DaemonClient
    let session: Session
    var body: some View { Text("Terminal: \(session.name)") }
}
