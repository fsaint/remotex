import Foundation

final class DaemonClient {
    enum ClientError: Swift.Error {
        case unauthorized
        case notFound
        case serverError(Int)
        case decodeFailed(Swift.Error)
    }

    private let credentials: Credentials
    private let session: URLSession
    private let decoder: JSONDecoder

    init(credentials: Credentials, session: URLSession = .shared) {
        self.credentials = credentials
        self.session = session
        self.decoder = JSONDecoder()
        self.decoder.dateDecodingStrategy = .iso8601
    }

    private var baseURL: String {
        "http://\(credentials.host):7654"
    }

    private func authorizedRequest(path: String, method: String = "GET") -> URLRequest {
        var req = URLRequest(url: URL(string: "\(baseURL)\(path)")!)
        req.httpMethod = method
        req.setValue("Bearer \(credentials.apiKey)", forHTTPHeaderField: "Authorization")
        return req
    }

    func listSessions() async throws -> [Session] {
        let req = authorizedRequest(path: "/sessions")
        let (data, response) = try await session.data(for: req)
        try validate(response)
        do {
            return try decoder.decode([Session].self, from: data)
        } catch {
            throw ClientError.decodeFailed(error)
        }
    }

    func requestConnect(sessionName: String) async throws -> ConnectInfo {
        let req = authorizedRequest(path: "/sessions/\(sessionName)/connect", method: "POST")
        let (data, response) = try await session.data(for: req)
        try validate(response)
        do {
            return try decoder.decode(ConnectInfo.self, from: data)
        } catch {
            throw ClientError.decodeFailed(error)
        }
    }

    private func validate(_ response: URLResponse) throws {
        guard let http = response as? HTTPURLResponse else { return }
        switch http.statusCode {
        case 200...299: return
        case 401: throw ClientError.unauthorized
        case 404: throw ClientError.notFound
        default: throw ClientError.serverError(http.statusCode)
        }
    }
}
