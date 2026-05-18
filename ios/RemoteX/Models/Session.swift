import Foundation

struct Session: Codable, Identifiable, Hashable {
    enum Status: String, Codable, Hashable {
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

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        name      = try c.decode(String.self, forKey: .name)
        tmuxPID   = try c.decodeIfPresent(Int.self,    forKey: .tmuxPID)    ?? 0
        moshPID   = try c.decodeIfPresent(Int.self,    forKey: .moshPID)   ?? 0
        moshPort  = try c.decodeIfPresent(Int.self,    forKey: .moshPort)  ?? 0
        moshKey   = try c.decodeIfPresent(String.self, forKey: .moshKey)   ?? ""
        startedAt = try c.decodeIfPresent(Date.self,   forKey: .startedAt) ?? Date()
        status    = try c.decode(Status.self, forKey: .status)
    }
}
