package persist

import (
	"testing"
)

func TestModelTableName(t *testing.T) {
	var m DistanceMatrixEdge
	if m.TableName() != "distance_matrix_edge" {
		t.Fatal(m.TableName())
	}
}
