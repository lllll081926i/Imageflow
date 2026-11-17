package ui

// rail_migration_test.go pins COMP-02: a sidebar order saved by v3.1.0 must
// survive the upgrade into stable tool identifiers, and the user's collage
// parameters must come through untouched.

import (
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// TestLegacyRailOrderMigratesThroughTheIdentityTable pins the exact migration M0
// requires: the stored positions refer to the v3.1.0 tool list, so they are mapped
// through that table and not through the current one.
func TestLegacyRailOrderMigratesThroughTheIdentityTable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", dir+"/settings.json")

	// The fixture is v3.1.0's list with the last row moved to the front and the
	// collage (index 5) moved to position 2.
	legacy := []int{13, 0, 5, 1, 2, 3, 4, 6, 7, 8, 9, 10, 11, 12}
	saved := settings.Default()
	saved.RailOrder = legacy
	saved.Collage.LongEdge = 2048
	saved.Collage.WatermarkText = "老水印"
	if err := settings.Save(saved); err != nil {
		t.Fatal(err)
	}

	app := NewApp(settings.Load())
	order := app.railOrderSnapshot()
	want := []string{
		ToolCompress, ToolConvert, ToolCollage, ToolResize, ToolCrop, ToolGrid,
		ToolLayoutPage, ToolStitch, ToolWatermark, ToolPalette, ToolIcon, ToolPDF,
		ToolGIF, ToolRichText,
	}
	if len(order) != len(app.Tools) {
		t.Fatalf("order length = %d, want %d", len(order), len(app.Tools))
	}
	for i, id := range want {
		if got := app.Tools[order[i]].ID(); got != id {
			t.Fatalf("row %d = %q, want %q", i, got, id)
		}
	}
	// The new pages follow the migrated rows in their natural order.
	for i := len(want); i < len(order); i++ {
		if app.Tools[order[i]].ID() == "" {
			t.Fatalf("row %d has no id", i)
		}
	}
	// The collage parameters came through.
	collage, ok := app.ActiveTool(), false
	for _, tool := range app.Tools {
		if tool.ID() == ToolCollage {
			ok = true
			break
		}
	}
	if !ok || collage == nil {
		t.Fatal("collage page missing")
	}
	persisted := app.Session.SettingsSnapshot().Collage
	if persisted.LongEdge != 2048 || persisted.WatermarkText != "老水印" {
		t.Fatalf("collage lost its parameters: %+v", persisted)
	}
}

// TestRailOrderMigrationIgnoresOutOfRangePositions pins the guard on a hand-edited
// or truncated file: an impossible position is dropped instead of panicking.
func TestRailOrderMigrationIgnoresOutOfRangePositions(t *testing.T) {
	ids := migrateRailOrderIDs([]int{99, -3, 0, 0, 1})
	if len(ids) != 2 {
		t.Fatalf("ids = %v, want the two valid, de-duplicated entries", ids)
	}
	if ids[0] != ToolConvert || ids[1] != ToolResize {
		t.Fatalf("ids = %v", ids)
	}
}

// TestPersistedOrderClearsTheLegacyList pins that after one save only the
// identifiers remain: leaving the index list behind would let a later migration
// replay it over the new order.
func TestPersistedOrderClearsTheLegacyList(t *testing.T) {
	app := newTestApp(t)
	app.Settings.RailOrder = []int{0, 1, 2}
	app.persistRailOrder()
	loaded := settings.Load()
	if len(loaded.RailOrder) != 0 {
		t.Fatalf("legacy list survived: %v", loaded.RailOrder)
	}
	if len(loaded.RailOrderIDs) != len(app.Tools) {
		t.Fatalf("ids = %d, want %d", len(loaded.RailOrderIDs), len(app.Tools))
	}
}

// TestEveryNewPageHasAStableID pins that the identifiers are unique and that the
// legacy table still matches the fourteen original rows, because a mismatch would
// silently reinterpret every stored position.
func TestEveryNewPageHasAStableID(t *testing.T) {
	app := newTestApp(t)
	seen := map[string]int{}
	for i, tool := range app.Tools {
		id := tool.ID()
		if id == "" {
			t.Fatalf("tool %d (%s) has an empty id", i, tool.Name())
		}
		if other, ok := seen[id]; ok {
			t.Fatalf("%q and %q share the id %q", app.Tools[other].Name(), tool.Name(), id)
		}
		seen[id] = i
	}
	if len(legacyToolIDs) != 14 {
		t.Fatalf("the legacy table has %d rows, want 14", len(legacyToolIDs))
	}
	for i, id := range legacyToolIDs {
		if app.Tools[i].ID() != id {
			t.Fatalf("legacy row %d = %q, but the tool there is %q", i, id, app.Tools[i].ID())
		}
	}
}
