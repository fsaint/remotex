import UIKit

struct TerminalSize {
    let cols: Int
    let rows: Int
}

enum TerminalSizeHelper {
    /// Computes terminal cols/rows for the given bounds using a monospace font.
    static func size(for bounds: CGRect, fontSize: CGFloat = 14) -> TerminalSize {
        let charWidth  = fontSize * 0.601
        let lineHeight = fontSize * 1.2
        let cols = max(80, Int(bounds.width  / charWidth))
        let rows = max(24, Int(bounds.height / lineHeight))
        return TerminalSize(cols: cols, rows: rows)
    }
}
