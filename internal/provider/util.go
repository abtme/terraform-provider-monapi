package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abtme/terraform-provider-monapi/internal/client"
)

// clientFrom gets the configured client in a resource's Configure.
func clientFrom(data any, diags *diag.Diagnostics) *client.Client {
	if data == nil {
		return nil // provider not configured yet (validation)
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Internal error", fmt.Sprintf("unexpected provider data %T", data))
	}
	return c
}

// importByID lets `terraform import` take the object's id.
func importByID(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func stringList(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

func stringSet(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	out := []string{}
	diags.Append(s.ElementsAs(ctx, &out, false)...)
	return out
}

func ints(ctx context.Context, l types.List, diags *diag.Diagnostics) []int {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var v []int64
	diags.Append(l.ElementsAs(ctx, &v, false)...)
	out := make([]int, len(v))
	for i, n := range v {
		out[i] = int(n)
	}
	return out
}

func listOfStrings(v []string) types.List {
	elems := make([]attr.Value, len(v))
	for i, s := range v {
		elems[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, elems)
}

func setOfStrings(v []string) types.Set {
	elems := make([]attr.Value, len(v))
	for i, s := range v {
		elems[i] = types.StringValue(s)
	}
	return types.SetValueMust(types.StringType, elems)
}

func listOfInts(v []int) types.List {
	elems := make([]attr.Value, len(v))
	for i, n := range v {
		elems[i] = types.Int64Value(int64(n))
	}
	return types.ListValueMust(types.Int64Type, elems)
}

// apiError turns a client error into a diagnostic.
func apiError(diags *diag.Diagnostics, what string, err error) {
	diags.AddError("monapi: "+what, err.Error())
}
