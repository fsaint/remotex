import SwiftUI
import AVFoundation

struct SetupView: View {
    let router: AppRouter
    @State private var error: String?
    @State private var isScanning = true
    @State private var showPasteSheet = false
    @State private var pasteText = ""

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

            Button("Paste pairing JSON instead") {
                pasteText = UIPasteboard.general.string ?? ""
                showPasteSheet = true
            }
            .font(.footnote)
            .foregroundStyle(.secondary)
        }
        .padding()
        .sheet(isPresented: $showPasteSheet) {
            PasteSetupSheet(text: $pasteText) { json in
                showPasteSheet = false
                handleQR(json)
            }
        }
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

struct PasteSetupSheet: View {
    @Binding var text: String
    let onConfirm: (String) -> Void

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 12) {
                Text("Paste the JSON shown by `remotex setup` on your Mac.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                TextEditor(text: $text)
                    .font(.system(.footnote, design: .monospaced))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(8)
                    .background(Color(.secondarySystemBackground))
                    .clipShape(RoundedRectangle(cornerRadius: 8))
            }
            .padding()
            .navigationTitle("Paste Pairing JSON")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Pair") { onConfirm(text) }
                        .disabled(text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { onConfirm("") }
                }
            }
        }
    }
}

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
    private let captureSession = AVCaptureSession()
    private var previewLayer: AVCaptureVideoPreviewLayer!

    override func layoutSubviews() {
        super.layoutSubviews()
        previewLayer?.frame = bounds
    }

    func startScanning() {
        guard let device = AVCaptureDevice.default(for: .video),
              let input = try? AVCaptureDeviceInput(device: device),
              captureSession.canAddInput(input) else { return }

        captureSession.addInput(input)
        let output = AVCaptureMetadataOutput()
        captureSession.addOutput(output)
        output.setMetadataObjectsDelegate(self, queue: .main)
        output.metadataObjectTypes = [.qr]

        previewLayer = AVCaptureVideoPreviewLayer(session: captureSession)
        previewLayer.videoGravity = .resizeAspectFill
        previewLayer.frame = bounds
        layer.insertSublayer(previewLayer, at: 0)

        DispatchQueue.global(qos: .userInitiated).async { self.captureSession.startRunning() }
    }

    func metadataOutput(_ output: AVCaptureMetadataOutput,
                        didOutput objects: [AVMetadataObject],
                        from connection: AVCaptureConnection) {
        guard let obj = objects.first as? AVMetadataMachineReadableCodeObject,
              let string = obj.stringValue else { return }
        captureSession.stopRunning()
        onScan?(string)
    }
}
