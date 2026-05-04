import Foundation
import Security

final class KeychainStore {
    enum Error: Swift.Error, Equatable {
        case notFound
        case saveFailed(OSStatus)
        case decodeFailed
    }

    private let service = "com.remotex.app"
    private let account = "credentials"

    var isPaired: Bool {
        (try? load()) != nil
    }

    func save(_ creds: Credentials) throws {
        let data = try JSONEncoder().encode(creds)
        deleteCredentials()

        let query: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String:   data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]
        let status = SecItemAdd(query as CFDictionary, nil)
        if status != errSecSuccess {
            throw Error.saveFailed(status)
        }
    }

    func load() throws -> Credentials {
        let query: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String:  true,
            kSecMatchLimit as String:  kSecMatchLimitOne
        ]
        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        guard status == errSecSuccess, let data = result as? Data else {
            throw Error.notFound
        }
        guard let creds = try? JSONDecoder().decode(Credentials.self, from: data) else {
            throw Error.decodeFailed
        }
        return creds
    }

    @discardableResult
    func deleteCredentials() -> Bool {
        let query: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        return SecItemDelete(query as CFDictionary) == errSecSuccess
    }
}
