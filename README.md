# Introduce
This is demo driver-plugin for IoT Edge. Through this demo, you can learn how to develop a IoT Edge driver-plugin.

IoT Edge is a grpc server and a driver-plugin is a grpc client, They communicate via grpc connection. The protobuf file is located in protobuf directory. The file contains all the details of grpc communication.

# Project structure

## protobuf
It Contains the protobuf files. You can use it to generate grpc client codes.

## pkg/api
Some struct definations. You can define your own structs in this directory.

## pkg/device
A device is an abstraction of a physical device. Device is the source of data. It can be a modbus(tcp/rtu), opcua, opcda and other protocols. Each device collects device data according to the configuration

## pkg/devicescheduler
Device scheduler is responsible for communication with IoT Edge, and control the life cycle of the devices.

It accepts instructions from IoT Edge to control the life cycle of the device.

The device and the device scheduler communicate through queues.

# How to build
make 

# How to run
./bin/driver-plugin --edge-server=127.0.0.1 --edge-port=8090 --token=xxxxx
