package devicescheduler

import (
	"context"
	"driver-plugin/pkg/driverplugin"
	"fmt"
	"sync"
	"time"

	"github.com/wh8199/log"

	"driver-plugin/pkg/device"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type DeviceScheduler struct {
	// grpc://127.0.0.1:80?token=xxxxx
	IP     string
	Port   int
	Token  string
	Conn   *grpc.ClientConn
	Client driverplugin.DriverPluginClient
	Header context.Context

	DevicesLock sync.RWMutex
	Devices     map[string]*device.Device

	DeviceMetas map[string]*driverplugin.DeviceMeta

	DisConnChan chan struct{}

	Ctx        context.Context
	CancelFunc context.CancelFunc

	ReportMessageChan chan *driverplugin.ReportMessage
}

func NewDeviceScheduler(iotedgeServerip, token string, iotedgeServerPort int) (*DeviceScheduler, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &DeviceScheduler{
		IP:                iotedgeServerip,
		Port:              iotedgeServerPort,
		Token:             token,
		Devices:           make(map[string]*device.Device),
		DeviceMetas:       make(map[string]*driverplugin.DeviceMeta),
		DisConnChan:       make(chan struct{}),
		Ctx:               ctx,
		CancelFunc:        cancelFunc,
		ReportMessageChan: make(chan *driverplugin.ReportMessage),
	}, nil
}

func (d *DeviceScheduler) Start() error {
	if err := d.Connect(); err != nil {
		return err
	}

	d.startSchedule()

	return nil
}

func (d *DeviceScheduler) Stop() {
	if d.CancelFunc != nil {
		d.CancelFunc()
	}
}

// list all devices when startup
func (d *DeviceScheduler) FetchDeviceMetas() error {
	deviceMetas, err := d.Client.FetchDeviceMeta(d.Header, &driverplugin.DeviceMetaFetchRequest{})
	if err != nil {
		return err
	}

	// list all devicemetas,
	d.DeviceMetas = map[string]*driverplugin.DeviceMeta{}
	for _, deviceMeta := range deviceMetas.DeviceMetas {
		d.DeviceMetas[deviceMeta.Defination.DeviceID] = deviceMeta
	}

	for _, device := range d.Devices {
		device.Stop()
	}

	for _, deviceMeta := range deviceMetas.DeviceMetas {
		d.startDevice(deviceMeta)
	}

	return nil
}

func (d *DeviceScheduler) closeConnection() {
	if d.Conn != nil {
		d.Conn.Close()
		d.Conn = nil
	}

	if d.Client != nil {
		d.Client = nil
	}
}

// connect to iotedge
func (d *DeviceScheduler) Connect() error {
	address := fmt.Sprintf("%s:%d", d.IP, d.Port)
	md := metadata.Pairs("token", d.Token)

	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Error(err)
		return err
	}

	d.Conn = conn
	d.Client = driverplugin.NewDriverPluginClient(conn)
	d.Header = metadata.NewOutgoingContext(context.Background(), md)

	resp, err := d.Client.Connect(d.Header, &driverplugin.ConnectRequest{})
	if err != nil {
		log.Error(err)
		d.closeConnection()
		return err
	}

	if resp.StatusCode != driverplugin.StatusCode_StatusOK {
		d.closeConnection()
		return fmt.Errorf("fail to connect, status code is:%s, error message: %s", resp.StatusCode.String(), resp.ErrorMessage)
	}

	return d.OnConnect()
}

func (d *DeviceScheduler) startSchedule() {
	tickers := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-d.Ctx.Done():
			return
		case <-d.DisConnChan:
			d.OnConnectLost()
		case <-tickers.C:
			log.Debug("---start device scheduler---- \n")
			d.DevicesLock.Lock()
			for _, meta := range d.DeviceMetas {
				if meta.Defination.StopCollecting {
					continue
				}

				if d.Devices[meta.Defination.DeviceID] == nil {
					d.Devices[meta.Defination.DeviceID] = device.NewDevice(meta, d.ReportMessageChan)
					d.Devices[meta.Defination.DeviceID].Start()
				}
			}
			d.DevicesLock.Unlock()
		}
	}
}

func (d *DeviceScheduler) OnConnectLost() {
	log.Debug("connect to iotedge has lost, try to reconnect")
	d.closeConnection()
	for {
		if err := d.Connect(); err == nil {
			break
		}

		time.Sleep(5 * time.Second)
	}
}

func (d *DeviceScheduler) OnConnect() error {
	log.Info("connect to iotedge successfully")

	// get all devices from iotedge
	if err := d.FetchDeviceMetas(); err != nil {
		return err
	}

	lifeControlClient, err := d.Client.DeviceLifeControl(d.Header)
	if err != nil {
		return err
	}

	transformRawDataClient, err := d.Client.TransformRawData(d.Header)
	if err != nil {
		return err
	}

	// handle iotedge lifecontrol request
	go d.HandleLifeControl(lifeControlClient)
	go d.ReportMessageAndStatus(transformRawDataClient)
	return nil
}

func (d *DeviceScheduler) ReportMessageAndStatus(transformRawDataClient driverplugin.DriverPlugin_TransformRawDataClient) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.Ctx.Done():
			return nil
		case <-ticker.C:
			heartbeat := &driverplugin.HeartBeat{
				DeviceStatus: map[string]*driverplugin.DeviceStatus{},
			}

			d.DevicesLock.Lock()
			for _, device := range d.Devices {
				state, errorMessage := device.GetState()

				deviceID := device.GetDeivceID()
				heartbeat.DeviceStatus[deviceID] = &driverplugin.DeviceStatus{
					State: state,
					Error: errorMessage,
				}
			}
			d.DevicesLock.Unlock()

			if d.Client == nil {
				return nil
			}

			if _, err := d.Client.SendHeartBeat(d.Header, heartbeat); err != nil {
				log.Error(err)
				return err
			}

		case msg := <-d.ReportMessageChan:
			log.Debugf("receive msg %v", msg)
			if err := transformRawDataClient.Send(msg); err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return nil
}

// add device meta, only put device meta to deviceMetas
func (d *DeviceScheduler) addDevice(deviceMeta *driverplugin.DeviceMeta) {
	d.DevicesLock.Lock()
	d.DeviceMetas[deviceMeta.Defination.DeviceID] = deviceMeta
	d.DevicesLock.Unlock()
}

// create a device instance, and put it into devices
func (d *DeviceScheduler) startDevice(deviceMeta *driverplugin.DeviceMeta) {
	if deviceMeta.Defination.StopCollecting {
		log.Debugf("device %s is stop collect", deviceMeta.Defination.DeviceID)
		return
	}

	log.Debugf("device %s is start schedule ", deviceMeta.Defination.DeviceID)
	deviceID := deviceMeta.Defination.DeviceID
	d.DevicesLock.Lock()
	// update devicemeta
	d.DeviceMetas[deviceID] = deviceMeta
	// get olddevice
	oldDevice := d.Devices[deviceID]
	d.DevicesLock.Unlock()

	if oldDevice != nil {
		oldDevice.Stop()
	}

	d.DevicesLock.Lock()
	device := device.NewDevice(deviceMeta, d.ReportMessageChan)
	d.Devices[deviceID] = device
	d.DevicesLock.Unlock()

	device.Start()
}

// stop device
func (d *DeviceScheduler) stopDevice(deviceID string) {
	d.DevicesLock.Lock()
	device := d.Devices[deviceID]
	d.DevicesLock.Unlock()

	if device != nil {
		device.Stop()
	}
}

// stop device and remove it from devicemetas
func (d *DeviceScheduler) deleteDevice(deviceID string) {
	d.stopDevice(deviceID)

	d.DevicesLock.Lock()
	delete(d.DeviceMetas, deviceID)
	d.DevicesLock.Unlock()
}

// handle iotedge life cycle control request
func (d *DeviceScheduler) HandleLifeControl(lifeControlClient driverplugin.DriverPlugin_DeviceLifeControlClient) error {
	for {
		select {
		case <-d.Ctx.Done():
			return nil
		default:
			req, err := lifeControlClient.Recv()
			if err != nil {
				log.Error(err)
				d.DisConnChan <- struct{}{}
				return err
			}

			resp := driverplugin.Response{
				RequestID:  req.RequestID,
				StatusCode: driverplugin.StatusCode_StatusOK,
			}

			switch req.Command {
			case driverplugin.DeviceLifeControlRequest_AddDevice:
				// add device meta
				d.addDevice(req.Meta)
			case driverplugin.DeviceLifeControlRequest_DeleteDevice:
				// stop device collecting
				d.deleteDevice(req.Meta.Defination.DeviceID)
			case driverplugin.DeviceLifeControlRequest_RestartDevice, driverplugin.DeviceLifeControlRequest_StartDevice:
				// restart device
				// 1) stop device
				// 2) start new device
				d.startDevice(req.Meta)
			case driverplugin.DeviceLifeControlRequest_StopDevice:
				// stop device collecting
				d.stopDevice(req.Meta.Defination.DeviceID)
			default:
				resp.StatusCode = driverplugin.StatusCode_UnsupportLifeCycleRequest
			}

			if err := lifeControlClient.Send(&resp); err != nil {
				log.Error(err)
			}
		}
	}

	return fmt.Errorf("grpc client is closed")
}
