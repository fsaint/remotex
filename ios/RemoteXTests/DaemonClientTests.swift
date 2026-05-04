import XCTest
@testable import RemoteX

final class DaemonClientTests: XCTestCase {
    func testListSessionsDecodesCorrectly() async throws {
        let json = """
        [
          {"name":"work","tmux_pid":1,"mosh_pid":0,"mosh_port":0,"mosh_key":"",
           "started_at":"2026-05-03T10:00:00Z","status":"live"},
          {"name":"side","tmux_pid":2,"mosh_pid":0,"mosh_port":0,"mosh_key":"",
           "started_at":"2026-05-03T11:00:00Z","status":"dead"}
        ]
        """.data(using: .utf8)!

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        let sessions = try decoder.decode([Session].self, from: json)

        XCTAssertEqual(sessions.count, 2)
        XCTAssertEqual(sessions[0].name, "work")
        XCTAssertTrue(sessions[0].isLive)
        XCTAssertFalse(sessions[1].isLive)
    }

    func testConnectInfoDecodes() throws {
        let json = """
        {"host":"myhost.ts.net","port":60001,"key":"ABC123defGHI456jklMNO7"}
        """.data(using: .utf8)!
        let info = try JSONDecoder().decode(ConnectInfo.self, from: json)
        XCTAssertEqual(info.port, 60001)
        XCTAssertEqual(info.key, "ABC123defGHI456jklMNO7")
    }
}
