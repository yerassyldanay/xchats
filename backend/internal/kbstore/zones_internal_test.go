package kbstore

import (
	"strings"
	"testing"
)

func zone(ref, parent string, available bool, cost, days string) ZoneRow {
	return ZoneRow{Ref: ref, Name: ref, ZoneLevel: "city", ParentRef: parent,
		DeliveryAvailable: available, DeliveryCost: cost, DeliveryInDays: days}
}

func hasReason(reasons []string, substr string) bool {
	return strings.Contains(strings.Join(reasons, "; "), substr)
}

func TestZoneGateReasons_AvailableRequiresCostAndDays(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "", true, "", "")}, nil)
	if !hasReason(r, "delivery_cost and delivery_in_days are both required") {
		t.Fatalf("want a missing-cost/days reason, got %v", r)
	}
}

func TestZoneGateReasons_AvailableWithCostAndDaysOK(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "", true, "5 000 ₸", "1")}, []PolicyRow{
		{Lang: "*", OutsideZonesNote: "не доставляем"},
	})
	if len(r) != 0 {
		t.Fatalf("fully-specified available zone should pass, got %v", r)
	}
}

func TestZoneGateReasons_UnavailableMustBeBlank(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("baikonur", "", false, "5 000 ₸", "")}, nil)
	if !hasReason(r, "must both be blank") {
		t.Fatalf("want a must-be-blank reason, got %v", r)
	}
}

func TestZoneGateReasons_UnavailableBlankOK(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("baikonur", "", false, "", "")}, []PolicyRow{
		{Lang: "*", OutsideZonesNote: "не доставляем"},
	})
	if len(r) != 0 {
		t.Fatalf("fully-blank unavailable zone should pass, got %v", r)
	}
}

func TestZoneGateReasons_SelfParentRef(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "almaty", true, "1 ₸", "1")}, nil)
	if !hasReason(r, "cannot reference itself") {
		t.Fatalf("want a self-reference reason, got %v", r)
	}
}

func TestZoneGateReasons_DanglingParentRef(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "nowhere", true, "1 ₸", "1")}, nil)
	if !hasReason(r, "does not resolve to an existing zone") {
		t.Fatalf("want a dangling-parent reason, got %v", r)
	}
}

func TestZoneGateReasons_ParentRefCycle(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{
		zone("a", "b", true, "1 ₸", "1"),
		zone("b", "a", true, "1 ₸", "1"),
	}, []PolicyRow{{Lang: "*", OutsideZonesNote: "x"}})
	if !hasReason(r, "cycle") {
		t.Fatalf("want a cycle reason, got %v", r)
	}
}

func TestZoneGateReasons_ValidParentChainOK(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{
		zone("kazakhstan", "", true, "10 000 ₸", "3–4"),
		zone("almaty", "kazakhstan", true, "5 000 ₸", "1"),
	}, []PolicyRow{{Lang: "*", OutsideZonesNote: "не доставляем за пределами списка"}})
	if len(r) != 0 {
		t.Fatalf("valid two-level chain should pass, got %v", r)
	}
}

func TestZoneGateReasons_EmptyZonesSkipsPolicyChecks(t *testing.T) {
	// No zones at all: a flat, non-blank policy delivery answer is legal — the
	// zones feature is simply unused (aiprompt.Policies's doc comment).
	r := zoneGateReasons(nil, []PolicyRow{{Lang: "*", DeliveryCost: "1 500 ₸", DeliveryTime: "1–3 дня"}})
	if len(r) != 0 {
		t.Fatalf("no zones should skip every policy check, got %v", r)
	}
}

func TestZoneGateReasons_ZonesRequireBlankFlatPolicyFields(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "", true, "5 000 ₸", "1")}, []PolicyRow{
		{Lang: "*", DeliveryCost: "1 500 ₸", OutsideZonesNote: "не доставляем"},
	})
	if !hasReason(r, "delivery_cost and delivery_time must be blank") {
		t.Fatalf("want a flat-delivery-must-be-blank reason, got %v", r)
	}
}

func TestZoneGateReasons_ZonesRequireOutsideZonesNote(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "", true, "5 000 ₸", "1")}, []PolicyRow{
		{Lang: "*", OutsideZonesNote: ""},
	})
	if !hasReason(r, "outside_zones_note is required") {
		t.Fatalf("want an outside_zones_note-required reason, got %v", r)
	}
}

func TestZoneGateReasons_ZonesWithNoPolicyRowsAtAllIsBlocked(t *testing.T) {
	// Zero policy rows must not vacuously satisfy "every policy row is
	// compliant" — with no rows to check, the loop finds nothing to flag, so
	// this needs its own explicit check.
	r := zoneGateReasons([]ZoneRow{zone("almaty", "", true, "5 000 ₸", "1")}, nil)
	if !hasReason(r, "at least one policy row with outside_zones_note is required") {
		t.Fatalf("want a no-policy-rows reason when zones exist, got %v", r)
	}
}

func TestZoneGateReasons_EveryPolicyLangRowChecked(t *testing.T) {
	r := zoneGateReasons([]ZoneRow{zone("almaty", "", true, "5 000 ₸", "1")}, []PolicyRow{
		{Lang: "ru", OutsideZonesNote: "ok"},
		{Lang: "kk", OutsideZonesNote: ""},
	})
	if !hasReason(r, `policy "kk"`) {
		t.Fatalf("want the kk row flagged, got %v", r)
	}
	if hasReason(r, `policy "ru"`) {
		t.Fatalf("compliant ru row should not be flagged, got %v", r)
	}
}
