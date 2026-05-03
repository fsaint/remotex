import Foundation

struct Credentials: Codable {
    let host: String
    let apiKey: String
    let sshPrivateKey: String

    enum CodingKeys: String, CodingKey {
        case host
        case apiKey       = "api_key"
        case sshPrivateKey = "ssh_private_key"
    }
}
