package sqlite

import (
	"testing"

	"gorm.io/gorm/schema"
)

func TestDataTypeOfGeneratedColumn(t *testing.T) {
	dialector := Dialector{}
	tests := []struct {
		name  string
		field *schema.Field
		want  string
	}{
		{
			name:  "computed column renders a STORED generated column",
			field: &schema.Field{DataType: schema.Float, TagSettings: map[string]string{"GENERATED": "price * quantity"}},
			want:  "real GENERATED ALWAYS AS (price * quantity) STORED",
		},
		{
			name:  "computed expression keeps commas",
			field: &schema.Field{DataType: schema.String, TagSettings: map[string]string{"GENERATED": "coalesce(first_name, last_name)"}},
			want:  "text GENERATED ALWAYS AS (coalesce(first_name, last_name)) STORED",
		},
		{
			// `identity` is reserved for identity columns, which SQLite renders
			// through its native AUTOINCREMENT rather than a computed column.
			name:  "identity keyword is not treated as a computed column",
			field: &schema.Field{DataType: schema.Int, AutoIncrement: true, TagSettings: map[string]string{"GENERATED": "identity"}},
			want:  "integer PRIMARY KEY AUTOINCREMENT",
		},
		{
			name:  "identity with an explicit mode is also reserved",
			field: &schema.Field{DataType: schema.Int, AutoIncrement: true, TagSettings: map[string]string{"GENERATED": "identity always"}},
			want:  "integer PRIMARY KEY AUTOINCREMENT",
		},
		{
			name:  "a bare generated tag is ignored",
			field: &schema.Field{DataType: schema.Float, TagSettings: map[string]string{"GENERATED": "GENERATED"}},
			want:  "real",
		},
		{
			name:  "a lowercase generated expression is not mistaken for a bare tag",
			field: &schema.Field{DataType: schema.Float, TagSettings: map[string]string{"GENERATED": "generated"}},
			want:  "real GENERATED ALWAYS AS (generated) STORED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dialector.DataTypeOf(tt.field); got != tt.want {
				t.Errorf("DataTypeOf() = %q, want %q", got, tt.want)
			}
		})
	}
}
