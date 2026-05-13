import Foundation

final class DaemonClient {
    enum ClientError: Swift.Error {
        case unauthorized
        case notFound
        case serverError(Int)
        case decodeFailed(Swift.Error)
        case invalidURL
    }

    private let credentials: Credentials
    private let session: URLSession
    private let decoder: JSONDecoder

    init(credentials: Credentials, session: URLSession? = nil) {
        self.credentials = credentials

        if let session = session {
            self.session = session
        } else {
            let config = URLSessionConfiguration.default
            config.timeoutIntervalForRequest = 10
            self.session = URLSession(configuration: config)
        }

        self.decoder = JSONDecoder()

        // Go's time.Time marshals RFC3339Nano which may include fractional seconds.
        // Foundation's .iso8601 strategy rejects fractional seconds, so use a custom strategy.
        let dfFrac = ISO8601DateFormatter()
        dfFrac.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let dfNoFrac = ISO8601DateFormatter()
        dfNoFrac.formatOptions = [.withInternetDateTime]
        self.decoder.dateDecodingStrategy = .custom { dec in
            let container = try dec.singleValueContainer()
            let string = try container.decode(String.self)
            if let date = dfFrac.date(from: string) { return date }
            if let date = dfNoFrac.date(from: string) { return date }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Invalid ISO8601 date: \(string)")
        }
    }

    private var baseURL: String {
        "http://\(credentials.host):\(credentials.port)"
    }

    private func authorizedRequest(path: String, method: String = "GET") throws -> URLRequest {
        guard let url = URL(string: "\(baseURL)\(path)") else {
            throw ClientError.invalidURL
        }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("Bearer \(credentials.apiKey)", forHTTPHeaderField: "Authorization")
        return req
    }

    func listSessions() async throws -> [Session] {
        let req = try authorizedRequest(path: "/sessions")
        let (data, response) = try await session.data(for: req)
        try validate(response)
        do {
            return try decoder.decode([Session].self, from: data)
        } catch {
            throw ClientError.decodeFailed(error)
        }
    }

    func requestConnect(sessionName: String) async throws -> ConnectInfo {
        let encoded = sessionName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? sessionName
        let req = try authorizedRequest(path: "/sessions/\(encoded)/connect", method: "POST")
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
