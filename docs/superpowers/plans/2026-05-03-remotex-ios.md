# RemoteX iOS App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the RemoteX iOS app in Swift/SwiftUI that pairs with a Mac via QR code, lists tmux sessions from the remotex-daemon REST API, and connects to them via mosh with a full terminal emulator.

**Architecture:** SwiftUI app with three screens (Setup, Sessions, Terminal). DaemonClient handles REST calls to the Mac daemon over Tailscale. MoshSession wraps libmosh (from Blink Shell) to provide the network transport. SwiftTerm provides the VT100 terminal emulator. Credentials are stored in the iOS Keychain. The app routes to Setup on first launch, Sessions otherwise.

**Tech Stack:** Swift 5.9+, SwiftUI, SwiftTerm (SPM), AVFoundation (QR scanner), Security framework (Keychain), libmosh xcframework (built from Blink Shell open source).

**Prerequisite:** The Mac daemon (remotex-mac plan) must be running and `remotex setup` completed before testing the iOS app against a real Mac.

---

## File Structure

```
ios/
  RemoteX.xcodeproj
  RemoteX/
    App/
      RemoteXApp.swift          # @main, AppRouter injection
      AppRouter.swift           # ObservableObject: isPaired, activeScreen
    Models/
      Session.swift             # Codable Session struct matching daemon JSON
      Credentials.swift         # Codable Credentials struct
      ConnectInfo.swift         # Codable ConnectInfo {host, port, key}
    Services/
      KeychainStore.swift       # save/load Credentials from Keychain
      DaemonClient.swift        # REST: listSessions(), requestConnect()
      MoshSession.swift         # libmosh bridge: connect, disconnect, send input
    Screens/
      SetupView.swift           # QR scanner + pairing
      SessionsView.swift        # Session list, pull-to-refresh
      TerminalView.swift        # Full-screen SwiftTerm + MoshSession
    Utilities/
      TerminalSizeHelper.swift  # Compute cols/rows from UIScreen, post SIGWINCH
  RemoteXTests/
    KeychainStoreTests.swift
    DaemonClientTests.swift
    SessionTests.swift
  Frameworks/
    libmosh.xcframework         # Built from Blink Shell (see Task 2)
```

---

## Task 1: Xcode Project + Swift Package Dependencies

**Files:**
- Create: `ios/RemoteX.xcodeproj` (via Xcode)
- Packages: SwiftTerm

- [ ] **Step 1: Create Xcode project**

Open Xcode → File → New → Project → App
- Product Name: `RemoteX`
- Team: your personal team
- Interface: SwiftUI
- Language: Swift
- Uncheck "Include Tests" (we'll add manually)
- Save to `ios/` inside the repo

- [ ] **Step 2: Add SwiftTerm via Swift Package Manager**

In Xcode → File → Add Package Dependencies
Enter URL: `https://github.com/migueldeicaza/SwiftTerm`
Version: Up to Next Major from `1.0.0`
Add to target: RemoteX

- [ ] **Step 3: Add test target**

In Xcode → File → New → Target → Unit Testing Bundle
Name: `RemoteXTests`
Ensure it tests the `RemoteX` target.

- [ ] **Step 4: Create the directory structure**

In Xcode, create Groups (not folders) matching the file structure above:
`App`, `Models`, `Services`, `Screens`, `Utilities`

- [ ] **Step 5: Verify build**

Cmd+B in Xcode.
Expected: build succeeds with no errors.

- [ ] **Step 6: Commit**

```bash
cd ios
git add RemoteX.xcodeproj RemoteX/ RemoteXTests/
git commit -m "feat: xcode project scaffold with SwiftTerm package"
```

---

## Task 2: Build libmosh xcframework

**Files:**
- Create: `ios/Frameworks/libmosh.xcframework`

This task builds the mosh client library from Blink Shell's open source repository.

- [ ] **Step 1: Clone Blink Shell**

```bash
cd /tmp
git clone https://github.com/blinksh/blink.git --depth=1
cd blink
```

- [ ] **Step 2: Install build dependencies**

```bash
brew install automake libtool pkg-config protobuf
```

- [ ] **Step 3: Build the mosh framework**

Blink Shell provides build scripts. Follow their README for building the mosh framework:

```bash
cd /tmp/blink
# Initialize submodules (mosh is a submodule)
git submodule update --init --recursive

# Build mosh for iOS (check Blink's Makefile for the exact target)
make mosh-framework
```

If the above Makefile target doesn't exist, build manually:
```bash
# The mosh sources are in vendor/mosh/ or Libraries/mosh/
# Check Blink's build scripts in BuildUtils/ or scripts/
ls BuildUtils/
```

Follow Blink's actual build instructions — they change between versions. The output should be an `.xcframework` in the build artifacts.

- [ ] **Step 4: Copy xcframework into project**

```bash
cp -r /tmp/blink/build/libmosh.xcframework \
      /Users/fsaint/git/remotex/ios/Frameworks/
```

- [ ] **Step 5: Add xcframework to Xcode target**

In Xcode → RemoteX target → General → Frameworks, Libraries, and Embedded Content
Click `+` → Add Other → Add Files
Navigate to `ios/Frameworks/libmosh.xcframework`
Set embed to: "Embed & Sign"

- [ ] **Step 6: Verify it links**

Create a temporary Swift file to test the import:
```swift
// ios/RemoteX/App/MoshTest.swift (delete after verifying)
import Foundation
// import the mosh module name from Blink — check their headers
// e.g.: import MoshiOS or import mosh
```

Cmd+B — fix any linker errors by checking Blink Shell's module name and header.

- [ ] **Step 7: Commit**

```bash
git add ios/Frameworks/
git commit -m "feat: libmosh xcframework from Blink Shell"
```

**Note:** If building libmosh proves too complex for v1, use `SSHClient` + `autossh` as a fallback transport (SSH with auto-reconnect) and replace with mosh in v1.1. Mark this as a spike: timebox to 2 hours.

---

## Task 3: Models

**Files:**
- Create: `ios/RemoteX/Models/Session.swift`
- Create: `ios/RemoteX/Models/Credentials.swift`
- Create: `ios/RemoteX/Models/ConnectInfo.swift`

- [ ] **Step 1: Write failing tests**

```swift
// ios/RemoteXTests/SessionTests.swift
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

        let session = try JSONDecoder().decode(Session.self, from: json)
        XCTAssertEqual(session.name, "work")
        XCTAssertEqual(session.tmuxPID, 1234)
        XCTAssertEqual(session.status, .live)
        XCTAssertTrue(session.isLive)
    }

    func testDeadSessionIsNotLive() throws {
        let json = """
        {"name":"s","tmux_pid":1,"started_at":"2026-05-03T10:00:00Z","status":"dead"}
        """.data(using: .utf8)!
        let session = try JSONDecoder().decode(Session.self, from: json)
        XCTAssertFalse(session.isLive)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

In Xcode → Cmd+U (or `xcodebuild test -scheme RemoteX -destination 'platform=iOS Simulator,...'`)
Expected: compile error — `Session` not defined.

- [ ] **Step 3: Implement models**

```swift
// ios/RemoteX/Models/Session.swift
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
```

```swift
// ios/RemoteX/Models/Credentials.swift
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
```

```swift
// ios/RemoteX/Models/ConnectInfo.swift
import Foundation

struct ConnectInfo: Codable {
    let host: String
    let port: Int
    let key: String
}
```

- [ ] **Step 4: Run tests to verify they pass**

Cmd+U in Xcode.
Expected: `SessionTests` PASS

- [ ] **Step 5: Commit**

```bash
git add ios/RemoteX/Models/ ios/RemoteXTests/SessionTests.swift
git commit -m "feat: Session, Credentials, ConnectInfo models with JSON coding"
```

---

## Task 4: Keychain Store

**Files:**
- Create: `ios/RemoteX/Services/KeychainStore.swift`
- Create: `ios/RemoteXTests/KeychainStoreTests.swift`

- [ ] **Step 1: Write failing tests**

```swift
// ios/RemoteXTests/KeychainStoreTests.swift
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
        try store.save(Credentials(host: "h", apiKey: "k", sshPrivateKey: "p"))
        XCTAssertTrue(store.isPaired)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Expected: compile error — `KeychainStore` not defined.

- [ ] **Step 3: Implement KeychainStore**

```swift
// ios/RemoteX/Services/KeychainStore.swift
import Foundation
import Security

final class KeychainStore {
    enum Error: Swift.Error, Equatable {
        case notFound
        case saveFailed(OSStatus)
        case decodeFailed
    }

    private let service = "com.remotex.app"
    private let account = "credentials"

    var isPaired: Bool {
        (try? load()) != nil
    }

    func save(_ creds: Credentials) throws {
        let data = try JSONEncoder().encode(creds)
        deleteCredentials()

        let query: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String:   data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]
        let status = SecItemAdd(query as CFDictionary, nil)
        if status != errSecSuccess {
            throw Error.saveFailed(status)
        }
    }

    func load() throws -> Credentials {
        let query: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String:  true,
            kSecMatchLimit as String:  kSecMatchLimitOne
        ]
        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        guard status == errSecSuccess, let data = result as? Data else {
            throw Error.notFound
        }
        guard let creds = try? JSONDecoder().decode(Credentials.self, from: data) else {
            throw Error.decodeFailed
        }
        return creds
    }

    @discardableResult
    func deleteCredentials() -> Bool {
        let query: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        return SecItemDelete(query as CFDictionary) == errSecSuccess
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS (Keychain tests require a device or simulator with Keychain enabled)

If running in CI without Keychain entitlement, add `com.apple.security.keychain-access-groups` entitlement or use the simulator.

- [ ] **Step 5: Commit**

```bash
git add ios/RemoteX/Services/KeychainStore.swift ios/RemoteXTests/KeychainStoreTests.swift
git commit -m "feat: KeychainStore for saving/loading Credentials"
```

---

## Task 5: DaemonClient

**Files:**
- Create: `ios/RemoteX/Services/DaemonClient.swift`
- Create: `ios/RemoteXTests/DaemonClientTests.swift`

- [ ] **Step 1: Write failing tests**

```swift
// ios/RemoteXTests/DaemonClientTests.swift
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
```

- [ ] **Step 2: Run tests to verify they fail**

Expected: compile error — DaemonClient not imported (tests use models only, but DaemonClient must be defined).

- [ ] **Step 3: Implement DaemonClient**

```swift
// ios/RemoteX/Services/DaemonClient.swift
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
        var req = authorizedRequest(path: "/sessions/\(sessionName)/connect", method: "POST")
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
```

- [ ] **Step 4: Run tests to verify they pass**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ios/RemoteX/Services/DaemonClient.swift ios/RemoteXTests/DaemonClientTests.swift
git commit -m "feat: DaemonClient with listSessions and requestConnect"
```

---

## Task 6: AppRouter

**Files:**
- Create: `ios/RemoteX/App/AppRouter.swift`
- Create: `ios/RemoteX/App/RemoteXApp.swift`

- [ ] **Step 1: Implement AppRouter**

```swift
// ios/RemoteX/App/AppRouter.swift
import SwiftUI

@MainActor
final class AppRouter: ObservableObject {
    @Published var isPaired: Bool
    private let keychain: KeychainStore

    init(keychain: KeychainStore = KeychainStore()) {
        self.keychain = keychain
        self.isPaired = keychain.isPaired
    }

    func completePairing(with credentials: Credentials) {
        try? keychain.save(credentials)
        isPaired = true
    }

    func credentials() -> Credentials? {
        try? keychain.load()
    }
}
```

- [ ] **Step 2: Implement app entry point**

```swift
// ios/RemoteX/App/RemoteXApp.swift
import SwiftUI

@main
struct RemoteXApp: App {
    @StateObject private var router = AppRouter()
    private let keychain = KeychainStore()

    var body: some Scene {
        WindowGroup {
            if router.isPaired {
                if let creds = router.credentials() {
                    SessionsView(client: DaemonClient(credentials: creds))
                        .environment(\.keychain, keychain)
                }
            } else {
                SetupView(router: router)
            }
        }
    }
}
```

- [ ] **Step 3: Build to verify**

Cmd+B
Expected: compile errors only on missing `SetupView` and `SessionsView` — add stub views temporarily:

```swift
// Temporary stubs to allow building — will be replaced in Tasks 7 and 8
struct SetupView: View {
    let router: AppRouter
    var body: some View { Text("Setup") }
}
struct SessionsView: View {
    let client: DaemonClient
    var body: some View { Text("Sessions") }
}
```

- [ ] **Step 4: Commit**

```bash
git add ios/RemoteX/App/
git commit -m "feat: AppRouter and app entry point with pairing-based routing"
```

---

## Task 7: SetupView (QR Scanner)

**Files:**
- Modify: `ios/RemoteX/Screens/SetupView.swift` (replace stub)

- [ ] **Step 1: Implement SetupView with QR scanner**

```swift
// ios/RemoteX/Screens/SetupView.swift
import SwiftUI
import AVFoundation

struct SetupView: View {
    let router: AppRouter
    @State private var error: String?
    @State private var isScanning = true

    var body: some View {
        VStack(spacing: 24) {
            Text("Pair with Your Mac")
                .font(.largeTitle.bold())

            Text("Run `remotex setup` on your Mac, then scan the QR code it shows.")
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
                .padding(.horizontal)

            if isScanning {
                QRScannerView { result in
                    handleQR(result)
                }
                .frame(height: 320)
                .clipShape(RoundedRectangle(cornerRadius: 16))
                .padding(.horizontal)
            }

            if let error {
                Text(error)
                    .foregroundStyle(.red)
                    .font(.footnote)
            }
        }
        .padding()
    }

    private func handleQR(_ string: String) {
        isScanning = false
        guard
            let data = string.data(using: .utf8),
            let creds = try? JSONDecoder().decode(Credentials.self, from: data)
        else {
            error = "Invalid QR code. Make sure you scanned the RemoteX pairing code."
            isScanning = true
            return
        }
        router.completePairing(with: creds)
    }
}
```

- [ ] **Step 2: Implement QRScannerView**

```swift
// ios/RemoteX/Screens/SetupView.swift — append below SetupView

struct QRScannerView: UIViewRepresentable {
    let onScan: (String) -> Void

    func makeUIView(context: Context) -> QRPreviewView {
        let view = QRPreviewView()
        view.onScan = onScan
        view.startScanning()
        return view
    }

    func updateUIView(_ uiView: QRPreviewView, context: Context) {}
}

final class QRPreviewView: UIView, AVCaptureMetadataOutputObjectsDelegate {
    var onScan: ((String) -> Void)?
    private let session = AVCaptureSession()
    private var previewLayer: AVCaptureVideoPreviewLayer!

    override func layoutSubviews() {
        super.layoutSubviews()
        previewLayer?.frame = bounds
    }

    func startScanning() {
        guard let device = AVCaptureDevice.default(for: .video),
              let input = try? AVCaptureDeviceInput(device: device),
              session.canAddInput(input) else { return }

        session.addInput(input)
        let output = AVCaptureMetadataOutput()
        session.addOutput(output)
        output.setMetadataObjectsDelegate(self, queue: .main)
        output.metadataObjectTypes = [.qr]

        previewLayer = AVCaptureVideoPreviewLayer(session: session)
        previewLayer.videoGravity = .resizeAspectFill
        previewLayer.frame = bounds
        layer.insertSublayer(previewLayer, at: 0)

        DispatchQueue.global(qos: .userInitiated).async { self.session.startRunning() }
    }

    func metadataOutput(_ output: AVCaptureMetadataOutput,
                        didOutput objects: [AVMetadataObject],
                        from connection: AVCaptureConnection) {
        guard let obj = objects.first as? AVMetadataMachineReadableCodeObject,
              let string = obj.stringValue else { return }
        session.stopRunning()
        onScan?(string)
    }
}
```

Add camera usage description to `Info.plist`:
```
NSCameraUsageDescription → "RemoteX needs camera access to scan the pairing QR code."
```

- [ ] **Step 3: Build and run on simulator**

Cmd+R on simulator.
Expected: Setup screen shown (camera won't work on simulator — test QR parsing logic manually with a unit test).

- [ ] **Step 4: Add unit test for QR parsing**

```swift
// ios/RemoteXTests/SetupViewTests.swift
import XCTest
@testable import RemoteX

final class SetupViewTests: XCTestCase {
    func testValidQRDecodesCredentials() throws {
        let payload = """
        {"host":"myhost.ts.net","api_key":"abc123","ssh_private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\\ntest\\n-----END OPENSSH PRIVATE KEY-----"}
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
```

- [ ] **Step 5: Run test**

Cmd+U
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add ios/RemoteX/Screens/SetupView.swift ios/RemoteXTests/SetupViewTests.swift
git commit -m "feat: SetupView with QR scanner and Credentials parsing"
```

---

## Task 8: SessionsView

**Files:**
- Modify: `ios/RemoteX/Screens/SessionsView.swift` (replace stub)

- [ ] **Step 1: Implement SessionsView**

```swift
// ios/RemoteX/Screens/SessionsView.swift
import SwiftUI

struct SessionsView: View {
    let client: DaemonClient
    @State private var sessions: [Session] = []
    @State private var isLoading = false
    @State private var error: String?
    @State private var selectedSession: Session?

    var body: some View {
        NavigationStack {
            Group {
                if isLoading && sessions.isEmpty {
                    ProgressView("Loading sessions...")
                } else if let error, sessions.isEmpty {
                    VStack(spacing: 16) {
                        Image(systemName: "exclamationmark.triangle")
                            .font(.largeTitle)
                            .foregroundStyle(.orange)
                        Text(error)
                            .multilineTextAlignment(.center)
                            .foregroundStyle(.secondary)
                        Button("Retry") { Task { await loadSessions() } }
                            .buttonStyle(.bordered)
                    }
                    .padding()
                } else {
                    List(sessions) { session in
                        SessionRow(session: session)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                if session.isLive {
                                    selectedSession = session
                                }
                            }
                    }
                    .refreshable { await loadSessions() }
                }
            }
            .navigationTitle("Sessions")
            .navigationDestination(item: $selectedSession) { session in
                TerminalView(client: client, session: session)
                // keychain flows through environment automatically
            }
        }
        .task { await loadSessions() }
    }

    private func loadSessions() async {
        isLoading = true
        error = nil
        do {
            sessions = try await client.listSessions()
        } catch {
            self.error = "Cannot reach Mac: \(error.localizedDescription)"
        }
        isLoading = false
    }
}

struct SessionRow: View {
    let session: Session

    var body: some View {
        HStack {
            Circle()
                .fill(session.isLive ? Color.green : Color.gray)
                .frame(width: 10, height: 10)
            VStack(alignment: .leading, spacing: 2) {
                Text(session.name)
                    .font(.headline)
                    .foregroundStyle(session.isLive ? .primary : .secondary)
                Text("Started \(session.startedAt.formatted(.relative(presentation: .named)))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if session.isLive {
                Image(systemName: "chevron.right")
                    .foregroundStyle(.tertiary)
            }
        }
        .padding(.vertical, 4)
    }
}
```

- [ ] **Step 2: Verify SessionsView builds**

Cmd+B — fix any compile errors.

- [ ] **Step 3: Commit**

```bash
git add ios/RemoteX/Screens/SessionsView.swift
git commit -m "feat: SessionsView with session list, status indicators, pull-to-refresh"
```

---

## Task 9: MoshSession (libmosh Bridge)

**Files:**
- Create: `ios/RemoteX/Services/MoshSession.swift`

This task bridges libmosh to Swift. The exact API depends on how Blink Shell exposes libmosh — check their Swift/ObjC headers after completing Task 2.

- [ ] **Step 1: Examine Blink Shell's mosh Swift interface**

After building libmosh.xcframework, check its headers:
```bash
find ios/Frameworks/libmosh.xcframework -name "*.h" | xargs grep -l "connect\|MoshSession\|MoshParams"
```

Note the actual class/function names — they will differ from the stubs below.

- [ ] **Step 2: Implement MoshSession wrapper**

```swift
// ios/RemoteX/Services/MoshSession.swift
import Foundation
// import the actual mosh module — replace with real module name from Task 2
// e.g.: import MoshiOS

protocol TerminalOutputHandler: AnyObject {
    func didReceiveOutput(_ data: Data)
    func didDisconnect(error: Error?)
}

final class MoshSession {
    enum SessionError: Swift.Error {
        case connectionFailed(String)
        case alreadyConnected
    }

    weak var outputHandler: TerminalOutputHandler?
    private var isConnected = false

    // NOTE: Replace the body of connect() with actual libmosh API calls
    // after examining the framework headers in Task 9 Step 1.
    func connect(info: ConnectInfo, sshPrivateKey: String,
                 cols: Int, rows: Int) async throws {
        guard !isConnected else { throw SessionError.alreadyConnected }

        // Pseudocode — replace with actual libmosh calls:
        // let params = MoshParams(
        //     host: info.host,
        //     port: UInt16(info.port),
        //     key: info.key,
        //     sshKey: sshPrivateKey,
        //     cols: UInt16(cols),
        //     rows: UInt16(rows)
        // )
        // let session = MoshClient(params: params)
        // session.outputCallback = { [weak self] data in
        //     self?.outputHandler?.didReceiveOutput(data)
        // }
        // try session.connect()
        isConnected = true
    }

    func send(_ data: Data) {
        // Replace with actual libmosh input method:
        // moshClient?.writeInput(data)
    }

    func resize(cols: Int, rows: Int) {
        // Replace with actual libmosh resize method:
        // moshClient?.resize(cols: UInt16(cols), rows: UInt16(rows))
    }

    func disconnect() {
        // Replace with actual libmosh disconnect:
        // moshClient?.disconnect()
        isConnected = false
        outputHandler?.didDisconnect(error: nil)
    }
}
```

- [ ] **Step 3: Write integration note**

The actual mosh connection cannot be unit-tested without a real mosh-server. Integration testing happens in Task 11 (end-to-end). The wrapper above isolates all libmosh calls to one file, making it easy to swap the transport if needed.

- [ ] **Step 4: Commit**

```bash
git add ios/RemoteX/Services/MoshSession.swift
git commit -m "feat: MoshSession wrapper isolating libmosh bridge"
```

---

## Task 10: TerminalView

**Files:**
- Modify: `ios/RemoteX/Screens/TerminalView.swift` (replace stub)
- Create: `ios/RemoteX/Utilities/TerminalSizeHelper.swift`

- [ ] **Step 1: Implement TerminalSizeHelper**

```swift
// ios/RemoteX/Utilities/TerminalSizeHelper.swift
import UIKit

struct TerminalSize {
    let cols: Int
    let rows: Int
}

enum TerminalSizeHelper {
    /// Computes terminal cols/rows for the given view bounds using a monospace font.
    static func size(for bounds: CGRect, fontSize: CGFloat = 14) -> TerminalSize {
        // Approximate monospace character size at given font size
        let charWidth  = fontSize * 0.601   // ~60% of point size for monospace
        let lineHeight = fontSize * 1.2

        let cols = max(80, Int(bounds.width  / charWidth))
        let rows = max(24, Int(bounds.height / lineHeight))
        return TerminalSize(cols: cols, rows: rows)
    }
}
```

- [ ] **Step 2: Write failing test for TerminalSizeHelper**

```swift
// ios/RemoteXTests/TerminalSizeHelperTests.swift
import XCTest
@testable import RemoteX

final class TerminalSizeHelperTests: XCTestCase {
    func testMinimumSize() {
        let size = TerminalSizeHelper.size(for: CGRect(x: 0, y: 0, width: 10, height: 10))
        XCTAssertGreaterThanOrEqual(size.cols, 80)
        XCTAssertGreaterThanOrEqual(size.rows, 24)
    }

    func testLargeScreen() {
        let size = TerminalSizeHelper.size(for: CGRect(x: 0, y: 0, width: 390, height: 844))
        XCTAssertGreaterThan(size.cols, 80)
        XCTAssertGreaterThan(size.rows, 24)
    }
}
```

- [ ] **Step 3: Run test to verify it passes**

Cmd+U
Expected: PASS

- [ ] **Step 4: Implement TerminalView**

```swift
// ios/RemoteX/Screens/TerminalView.swift
import SwiftUI
import SwiftTerm

struct TerminalView: View {
    let client: DaemonClient
    let session: Session
    @Environment(\.dismiss) private var dismiss
    @State private var connectInfo: ConnectInfo?
    @State private var error: String?
    @State private var isConnecting = true

    var body: some View {
        ZStack {
            if isConnecting {
                VStack(spacing: 16) {
                    ProgressView()
                    Text("Connecting to \(session.name)...")
                        .foregroundStyle(.secondary)
                }
            } else if let error {
                VStack(spacing: 16) {
                    Image(systemName: "xmark.circle")
                        .font(.largeTitle).foregroundStyle(.red)
                    Text(error)
                        .multilineTextAlignment(.center)
                    Button("Dismiss") { dismiss() }
                        .buttonStyle(.bordered)
                }
                .padding()
            } else if let info = connectInfo {
                MoshTerminalView(connectInfo: info)
                    .ignoresSafeArea()
            }
        }
        .navigationBarHidden(true)
        .gesture(DragGesture(minimumDistance: 50, coordinateSpace: .local)
            .onEnded { value in
                if value.translation.height > 100 { dismiss() }
            }
        )
        .task { await connect() }
    }

    private func connect() async {
        do {
            connectInfo = try await client.requestConnect(sessionName: session.name)
            isConnecting = false
        } catch {
            self.error = "Failed to connect: \(error.localizedDescription)"
            isConnecting = false
        }
    }
}

// UIViewRepresentable wrapping SwiftTerm's TerminalView + MoshSession
struct MoshTerminalView: UIViewRepresentable {
    let connectInfo: ConnectInfo
    @Environment(\.keychain) private var keychain

    func makeUIView(context: Context) -> TerminalViewWithMosh {
        let creds = (try? keychain.load())
        let view = TerminalViewWithMosh(connectInfo: connectInfo, sshKey: creds?.sshPrivateKey ?? "")
        return view
    }

    func updateUIView(_ uiView: TerminalViewWithMosh, context: Context) {}
}

final class TerminalViewWithMosh: UIView {
    private let terminalView: SwiftTerm.TerminalView
    private let moshSession = MoshSession()

    init(connectInfo: ConnectInfo, sshKey: String) {
        terminalView = SwiftTerm.TerminalView(frame: .zero)
        super.init(frame: .zero)

        addSubview(terminalView)
        terminalView.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            terminalView.topAnchor.constraint(equalTo: topAnchor),
            terminalView.bottomAnchor.constraint(equalTo: bottomAnchor),
            terminalView.leadingAnchor.constraint(equalTo: leadingAnchor),
            terminalView.trailingAnchor.constraint(equalTo: trailingAnchor),
        ])

        terminalView.delegate = self
        moshSession.outputHandler = self

        let size = TerminalSizeHelper.size(for: UIScreen.main.bounds)
        Task {
            try? await moshSession.connect(
                info: connectInfo,
                sshPrivateKey: sshKey,
                cols: size.cols,
                rows: size.rows
            )
        }
    }

    required init?(coder: NSCoder) { fatalError() }

    override func layoutSubviews() {
        super.layoutSubviews()
        let size = TerminalSizeHelper.size(for: bounds)
        moshSession.resize(cols: size.cols, rows: size.rows)
    }
}

extension TerminalViewWithMosh: SwiftTerm.TerminalViewDelegate {
    func send(source: SwiftTerm.TerminalView, data: ArraySlice<UInt8>) {
        moshSession.send(Data(data))
    }

    func scrolled(source: SwiftTerm.TerminalView, position: Double) {}
    func setTerminalTitle(source: SwiftTerm.TerminalView, title: String) {}
    func sizeChanged(source: SwiftTerm.TerminalView) {}
    func hostCurrentDirectoryUpdate(source: SwiftTerm.TerminalView, directory: String?) {}
    func requestOpenLink(source: SwiftTerm.TerminalView, link: String, params: [String: String]) {}
    func bell(source: SwiftTerm.TerminalView) {}
    func clipboardCopy(source: SwiftTerm.TerminalView, content: Data) {}
    func rangeChanged(source: SwiftTerm.TerminalView, startY: Int, endY: Int) {}
}

extension TerminalViewWithMosh: TerminalOutputHandler {
    func didReceiveOutput(_ data: Data) {
        let bytes = [UInt8](data)
        DispatchQueue.main.async {
            self.terminalView.feed(byteArray: ArraySlice(bytes))
        }
    }

    func didDisconnect(error: Error?) {
        // Terminal session ended — SwiftUI dismiss handled by parent
    }
}
```

Add keychain environment key:
```swift
// ios/RemoteX/App/AppRouter.swift — append
import SwiftUI

private struct KeychainKey: EnvironmentKey {
    static let defaultValue = KeychainStore()
}
extension EnvironmentValues {
    var keychain: KeychainStore {
        get { self[KeychainKey.self] }
        set { self[KeychainKey.self] = newValue }
    }
}
```

- [ ] **Step 5: Build and verify**

Cmd+B — fix any compile errors from SwiftTerm delegate protocol mismatch (check actual delegate methods in SwiftTerm docs/source).

- [ ] **Step 6: Run tests**

Cmd+U
Expected: `TerminalSizeHelperTests` PASS

- [ ] **Step 7: Commit**

```bash
git add ios/RemoteX/Screens/TerminalView.swift \
        ios/RemoteX/Utilities/TerminalSizeHelper.swift \
        ios/RemoteX/App/AppRouter.swift \
        ios/RemoteXTests/TerminalSizeHelperTests.swift
git commit -m "feat: TerminalView with SwiftTerm + MoshSession, SIGWINCH on resize"
```

---

## Task 11: Orientation Change (SIGWINCH)

The resize path is already handled in `TerminalViewWithMosh.layoutSubviews()` (Task 10), which calls `moshSession.resize(cols:rows:)` whenever the view lays out. No additional code is needed — SwiftUI triggers `layoutSubviews` on orientation change automatically.

- [ ] **Step 1: Verify resize path**

Run on a real device or simulator, rotate the screen while in the terminal, confirm the terminal reflows. If columns/rows don't update, add an explicit orientation change observer:

```swift
// Inside TerminalViewWithMosh.init — add after setting up terminalView:
NotificationCenter.default.addObserver(
    self,
    selector: #selector(orientationDidChange),
    name: UIDevice.orientationDidChangeNotification,
    object: nil
)

@objc private func orientationDidChange() {
    let size = TerminalSizeHelper.size(for: UIScreen.main.bounds)
    moshSession.resize(cols: size.cols, rows: size.rows)
}
```

- [ ] **Step 2: Commit if changes were made**

```bash
git add ios/RemoteX/Screens/TerminalView.swift
git commit -m "fix: explicit orientation observer for terminal resize"
```

---

## Task 12: Background / Foreground Lifecycle

- [ ] **Step 1: Implement background suspension in MoshSession**

```swift
// ios/RemoteX/Services/MoshSession.swift — add to init or call from TerminalView
private var backgroundObserver: NSObjectProtocol?
private var foregroundObserver: NSObjectProtocol?

func observeAppLifecycle() {
    backgroundObserver = NotificationCenter.default.addObserver(
        forName: UIApplication.didEnterBackgroundNotification,
        object: nil, queue: .main
    ) { [weak self] _ in
        // mosh-server maintains state; we just stop sending
        // mosh's own keepalive handles reconnection
    }

    foregroundObserver = NotificationCenter.default.addObserver(
        forName: UIApplication.willEnterForegroundNotification,
        object: nil, queue: .main
    ) { [weak self] _ in
        // Trigger a resize to refresh the terminal display
        // actual reconnect is handled by libmosh internally
    }
}

deinit {
    if let obs = backgroundObserver { NotificationCenter.default.removeObserver(obs) }
    if let obs = foregroundObserver { NotificationCenter.default.removeObserver(obs) }
}
```

Call `moshSession.observeAppLifecycle()` inside `TerminalViewWithMosh.init`.

mosh maintains the session server-side during background; libmosh will reconnect automatically when the app returns to foreground. No manual reconnection logic is needed.

- [ ] **Step 2: Build final**

```bash
xcodebuild build -scheme RemoteX -destination 'platform=iOS Simulator,name=iPhone 16'
```
Expected: BUILD SUCCEEDED

- [ ] **Step 3: Final commit**

```bash
git add ios/RemoteX/Services/MoshSession.swift
git commit -m "feat: app lifecycle observers in MoshSession for background handling"
```

---

## End-to-End Testing Checklist

Run these manually once both Mac and iOS plans are implemented:

- [ ] Run `remotex setup` on Mac — QR code printed
- [ ] Open RemoteX on iPhone — SetupView shown
- [ ] Scan QR code — routed to SessionsView
- [ ] Run `remotex new work` on Mac — appears in SessionsView on next refresh
- [ ] Tap "work" — TerminalView opens, connects to tmux session
- [ ] Type commands — appear in tmux session on Mac
- [ ] Switch from WiFi to cellular — session persists (mosh reconnects)
- [ ] Background app — session maintained on Mac
- [ ] Return to foreground — terminal resumes
- [ ] Rotate device — terminal reflows
- [ ] Run `remotex kill work` on Mac — session greyed out in SessionsView
