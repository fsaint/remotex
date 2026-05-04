import XCTest
@testable import RemoteX

final class TerminalSizeHelperTests: XCTestCase {
    func testMinimumSize() {
        let size = TerminalSizeHelper.size(for: CGRect(x: 0, y: 0, width: 10, height: 10))
        XCTAssertGreaterThanOrEqual(size.cols, 80)
        XCTAssertGreaterThanOrEqual(size.rows, 24)
    }

    func testLargeScreen() {
        // 800pt wide / (14 * 0.601) = ~95 cols, 844pt tall / (14 * 1.2) = ~50 rows
        let size = TerminalSizeHelper.size(for: CGRect(x: 0, y: 0, width: 800, height: 844))
        XCTAssertGreaterThan(size.cols, 80)
        XCTAssertGreaterThan(size.rows, 24)
    }
}
