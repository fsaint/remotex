import Foundation

struct Session: Codable, Identifiable {
    enum Status: String, Codable {
        case live, dead
    }

    var id: String { name }
    let name: String
    let tmuxPID: Int
    let moshPID: Int
    let moshPort: Int
    let moshKey: String
    let startedAt: Date
    let status: Status

    var isLive: Bool { status == .live }

    enum CodingKeys: String, CodingKey {
        case name
        case tmuxPID    = "tmux_pid"
        case moshPID    = "mosh_pid"
        case moshPort   = "mosh_port"
        case moshKey    = "mosh_key"
        case startedAt  = "started_at"
        case status
    }
}
