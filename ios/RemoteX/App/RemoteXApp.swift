import SwiftUI

@main
struct RemoteXApp: App {
    @StateObject private var router = AppRouter()
    private let keychain = KeychainStore()

    var body: some Scene {
        WindowGroup {
            if router.isPaired {
                if let creds = router.credentials() {
                    SessionsView(client: DaemonClient(credentials: creds), router: router)
                        .environment(\.keychain, keychain)
                }
            } else {
                SetupView(router: router)
            }
        }
    }
}
