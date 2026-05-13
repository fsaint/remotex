import Foundation

struct Credentials: Codable {
    let host: String
    let port: Int
    let apiKey: String

    enum CodingKeys: String, CodingKey {
        case host
        case port
        case apiKey = "api_key"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        host   = try c.decode(String.self, forKey: .host)
        port   = try c.decodeIfPresent(Int.self, forKey: .port) ?? 7654
        apiKey = try c.decode(String.self, forKey: .apiKey)
    }
}
