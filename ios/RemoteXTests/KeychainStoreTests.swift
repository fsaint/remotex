import XCTest
@testable import RemoteX

final class KeychainStoreTests: XCTestCase {
    let store = KeychainStore()

    override func setUp() {
        super.setUp()
        store.deleteCredentials()
    }

    func testSaveAndLoad() throws {
        let creds = Credentials(
            host: "myhost.ts.net",
            port: 7654,
            apiKey: "test-api-key-123",
            sshPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----"
        )
        XCTAssertNoThrow(try store.save(creds))

        let loaded = try store.load()
        XCTAssertEqual(loaded.host, creds.host)
        XCTAssertEqual(loaded.apiKey, creds.apiKey)
        XCTAssertEqual(loaded.sshPrivateKey, creds.sshPrivateKey)
    }

    func testLoadThrowsWhenEmpty() {
        XCTAssertThrowsError(try store.load()) { error in
            XCTAssertEqual(error as? KeychainStore.Error, .notFound)
        }
    }

    func testIsPaired() throws {
        XCTAssertFalse(store.isPaired)
        try store.save(Credentials(host: "h", port: 7654, apiKey: "k", sshPrivateKey: "p"))
        XCTAssertTrue(store.isPaired)
    }
}
