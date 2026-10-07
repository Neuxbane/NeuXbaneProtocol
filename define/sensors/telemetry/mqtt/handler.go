// @desc Sensor telemetry MQTT subscriber
// @topic sensors/+/telemetry
// @qos 1
package mqtt

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/mqtt"
)

// TelemetryData is the inbound sensor telemetry message.
type TelemetryData struct {
	SensorID    string  `json:"sensor_id" validate:"required"`
	Temperature float64 `json:"temperature" validate:"min=-50,max=150"`
	Timestamp   int64   `json:"timestamp" validate:"required"`
}

// TelemetryAck is the acknowledged sensor response.
type TelemetryAck struct {
	Acked    bool   `json:"acked" validate:"required"`
	SensorID string `json:"sensor_id" validate:"required"`
}

// Handler handles inbound MQTT sensor telemetry.
func Handler(ctx *mqtt.Ctx[TelemetryData, TelemetryAck]) (TelemetryAck, error) {
	data, err := ctx.Payload()
	if err != nil {
		return TelemetryAck{Acked: false, SensorID: "unknown"}, err
	}

	_ = ctx.Ack()

	return TelemetryAck{
		Acked:    true,
		SensorID: data.SensorID,
	}, nil
}
