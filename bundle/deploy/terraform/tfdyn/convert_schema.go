package tfdyn

import (
	"context"
	"fmt"

	"github.com/databricks/cli/bundle/internal/tf/schema"
	"github.com/databricks/cli/libs/dyn"
	"github.com/databricks/cli/libs/dyn/convert"
	"github.com/databricks/cli/libs/log"
)

func convertSchemaResource(ctx context.Context, vin dyn.Value) (dyn.Value, error) {
	// Normalize the output value to the target schema.
	vout, diags := convert.Normalize(schema.ResourceSchema{}, vin)
	for _, diag := range diags {
		log.Debugf(ctx, "schema normalization diagnostic: %s", diag.Summary)
	}

	return vout, nil
}

type schemaConverter struct{}

func (schemaConverter) Convert(ctx context.Context, key string, vin dyn.Value, out *schema.Resources) error {
	vout, err := convertSchemaResource(ctx, vin)
	if err != nil {
		return err
	}

	// Add the converted resource to the output.
	out.Schema[key] = vout.AsAny()

	// Configure grants for this resource.
	if grants := convertGrantsResource(ctx, vin); grants != nil {
		grants.Schema = fmt.Sprintf("${databricks_schema.%s.id}", key)
		out.Grants["schema_"+key] = grants
	}

	return nil
}

func init() {
	registerConverter("schemas", schemaConverter{})
}