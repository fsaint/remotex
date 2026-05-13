import SwiftUI

@MainActor
final class AppRouter: ObservableObject {
    @Published var isPaired: Bool
    private let keychain: KeychainStore

    init(keychain: KeychainStore = KeychainStore()) {
        self.keychain = keychain
        self.isPaired = keychain.isPaired
    }

    func completePairing(with credentials: Credentials) throws {
        try keychain.save(credentials)
        isPaired = true
    }

    func credentials() -> Credentials? {
        try? keychain.load()
    }

    func unpair() {
        keychain.deleteCredentials()
        isPaired = false
    }
}

private struct KeychainKey: EnvironmentKey {
    static let defaultValue = KeychainStore()
}
extension EnvironmentValues {
    var keychain: KeychainStore {
        get { self[KeychainKey.self] }
        set { self[KeychainKey.self] = newValue }
    }
}
