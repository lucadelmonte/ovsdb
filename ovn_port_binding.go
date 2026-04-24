package ovsdb

import "fmt"

// OvnPortBinding represents a row from the OVN Southbound Port_Binding
// table. Unlike OvnLogicalSwitchPort, it is not joined against the NB
// Logical_Switch_Port table, so it also exposes bindings that are not
// backed by an NB LSP (e.g. chassisredirect, patch, localnet, l3gateway).
type OvnPortBinding struct {
	UUID         string
	Type         string
	ChassisUUID  string
	DatapathUUID string
	LogicalPort  string
	TunnelKey    uint64
	ExternalIDs  map[string]string
}

// GetPortBindings returns every row in the OVN Southbound Port_Binding
// table. Bindings with no chassis assignment are included with an empty
// ChassisUUID.
func (cli *OvnClient) GetPortBindings() ([]*OvnPortBinding, error) {
	bindings := []*OvnPortBinding{}
	query := "SELECT _uuid, type, chassis, datapath, logical_port, tunnel_key, external_ids FROM Port_Binding"
	result, err := cli.Database.Southbound.Client.Transact(cli.Database.Southbound.Name, query)
	if err != nil {
		return nil, fmt.Errorf("%s: '%s' table error: %s", cli.Database.Southbound.Name, "Port_Binding", err)
	}
	for _, row := range result.Rows {
		pb := &OvnPortBinding{
			ExternalIDs: make(map[string]string),
		}
		if r, dt, err := row.GetColumnValue("_uuid", result.Columns); err != nil {
			continue
		} else {
			if dt != "string" {
				continue
			}
			pb.UUID = r.(string)
		}
		pb.Type = readOptionalString(row, result.Columns, "type")
		pb.ChassisUUID = readOptionalString(row, result.Columns, "chassis")
		pb.DatapathUUID = readOptionalString(row, result.Columns, "datapath")
		pb.LogicalPort = readOptionalString(row, result.Columns, "logical_port")
		if r, dt, err := row.GetColumnValue("tunnel_key", result.Columns); err == nil && dt == "integer" {
			pb.TunnelKey = uint64(r.(int64))
		}
		if r, dt, err := row.GetColumnValue("external_ids", result.Columns); err == nil && dt == "map[string]string" {
			pb.ExternalIDs = r.(map[string]string)
		}
		bindings = append(bindings, pb)
	}
	return bindings, nil
}

// readOptionalString returns the column as a string. An unset string
// column — or an unset foreign-key reference — comes back as an empty
// []string set, which is normalized to "". Callers can therefore
// distinguish "unbound/empty" from "missing column".
func readOptionalString(row Row, columns map[string]string, name string) string {
	r, dt, err := row.GetColumnValue(name, columns)
	if err != nil {
		return ""
	}
	switch dt {
	case "string":
		return r.(string)
	case "[]string":
		if s, ok := r.([]string); ok && len(s) > 0 {
			return s[0]
		}
	}
	return ""
}
