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
