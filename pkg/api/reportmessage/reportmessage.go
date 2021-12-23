package reportmessage

type ReportMessage struct {
	ProductID      string                    `json:"p_id,omitempty"`
	DeviceID       string                    `json:"d_id,omitempty"`
	DeviceServices map[string]*ReportService `json:"services,omitempty"`
	ReportTime     string                    `json:"ts,omitempty"`
}

type DeviceProperty struct {
	Value        interface{} `json:"value"`
	DataType     string      `json:"dataType,omitempty"`
	ItemType     string      `json:"itemType,omitempty"`
	Alias        string      `json:"alias,omitempty"`
	ErrorMessage string      `json:"errormessage"`
}

type ReportService struct {
	DeviceProperties map[string]*DeviceProperty `json:"properties"`
}
