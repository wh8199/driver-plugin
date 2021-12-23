package device

import "driver-plugin/pkg/driverplugin"

type Device struct {
	reportChan chan *driverplugin.ReportMessage

	DeviceID string
}

func NewDevice(meta *driverplugin.DeviceMeta, reportChan chan *driverplugin.ReportMessage) *Device {
	return &Device{
		reportChan: reportChan,
	}
}

func (d *Device) Start() {}

func (d *Device) Stop() {}

func (d *Device) GetState() (driverplugin.DeviceStatus_DeviceState, string) {
	return driverplugin.DeviceStatus_OnlineState, ""
}

func (d *Device) GetDeivceID() string {
	return d.DeviceID
}
