package main

import "testing"

func TestInventoryDriftControls(t *testing.T) {
	t.Parallel()

	expected := []byte(`[{"declaration":"Device","schema":"api/openapi.yaml#/components/schemas/Device"}]`)
	for _, actual := range [][]byte{
		[]byte(`[]`),
		[]byte(`[{"declaration":"Device","schema":"api/traits.openapi.yaml#/components/schemas/Device"}]`),
		[]byte(`[{"declaration":"Device","schema":"api/openapi.yaml#/components/schemas/Device","uses":[]}]`),
	} {
		err := compareInventory(expected, actual)
		if err == nil {
			t.Fatal("removed declaration, changed schema owner or stale use inventory accepted")
		}
	}

	err := compareInventory(expected, expected)
	if err != nil {
		t.Fatal(err)
	}
}
