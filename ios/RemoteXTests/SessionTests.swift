import XCTest
@testable import RemoteX

final class SessionTests: XCTestCase {
    func testSessionDecodesFromJSON() throws {
        let json = """
        {
            "name": "work",
            "tmux_pid": 1234,
            "mosh_pid": 0,
            "mosh_port": 0,
            "mosh_key": "",
            "started_at": "2026-05-03T10:00:00Z",
            "status": "live"
        }
        """.data(using: .utf8)!

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        let session = try decoder.decode(Session.self, from: json)
        XCTAssertEqual(session.name, "work")
        XCTAssertEqual(session.tmuxPID, 1234)
        XCTAssertEqual(session.status, .live)
        XCTAssertTrue(session.isLive)
    }

    func testDeadSessionIsNotLive() throws {
        let json = """
        {"name":"s","tmux_pid":1,"mosh_pid":0,"mosh_port":0,"mosh_key":"","started_at":"2026-05-03T10:00:00Z","status":"dead"}
        """.data(using: .utf8)!
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        let session = try decoder.decode(Session.self, from: json)
        XCTAssertFalse(session.isLive)
    }
}
