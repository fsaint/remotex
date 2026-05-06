import XCTest
@testable import RemoteX

final class SetupViewTests: XCTestCase {
    func testValidQRDecodesCredentials() throws {
        let payload = """
        {"host":"myhost.ts.net","port":7654,"api_key":"abc123","ssh_private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\\ntest\\n-----END OPENSSH PRIVATE KEY-----"}
        """
        let data = payload.data(using: .utf8)!
        let creds = try JSONDecoder().decode(Credentials.self, from: data)
        XCTAssertEqual(creds.host, "myhost.ts.net")
        XCTAssertEqual(creds.apiKey, "abc123")
    }

    func testInvalidQRFails() {
        let data = "not json".data(using: .utf8)!
        XCTAssertThrowsError(try JSONDecoder().decode(Credentials.self, from: data))
    }
}
