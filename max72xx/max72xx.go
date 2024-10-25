// Driver works for max7219 and 7221 with support for up to 8 devices in series
// Datasheet: https://datasheets.maximintegrated.com/en/ds/MAX7219-MAX7221.pdf
package max72xx

import (
	"errors"
	"machine"
)

const maxNumberOfDevices = 8

var (
	ErrInvalidData = errors.New("invalid data")
)

type Device struct {
	bus        machine.SPI
	cs         machine.Pin
	numDevices uint8 // Number of MAX7219 devices in series
}

// Command defines the data structure for sending commands to the MAX7219.
//
// The MAX7219 uses a 16-bit serial protocol divided into two 8-bit segments:
// - The first 8 bits (D15 to D8) represent the address of the register to write to.
// - The second 8 bits (D7 to D0) represent the data to be written to that register.
//
// Register Addresses:
// - 0x00: No-Op Register
// - 0x01 to 0x08: Digit Registers (control individual digits/segments)
// - 0x09: Decode Mode Register (configures decoding mode for digits)
// - 0x0A: Intensity Register (sets brightness level)
// - 0x0B: Scan Limit Register (sets number of digits to display)
// - 0x0C: Shutdown Register (controls shutdown mode)
// - 0x0F: Display Test Register (tests the display)
//
// Examples:
// To set the intensity to a medium level, send the following 16-bit data:
//
//	WriteCommand(0, Command{Register: REG_INTENSITY, Data: 10})
type Command struct {
	Register byte
	Data     byte
}

type Config struct {
	NumberOfDigits uint8
	Intensity      uint8
}

// NewDevice creates a new max7219 connection for multiple devices in series.
// The SPI wire must already be configured.
// The SPI frequency must not be higher than 10MHz.
// parameter cs: the datasheet also refers to this pin as "load" pin.
// parameter numDevices: number of MAX7219 devices connected in series (1-8)
func NewDevice(bus machine.SPI, cs machine.Pin, numDevices uint8) *Device {
	if numDevices < 1 && numDevices > maxNumberOfDevices {
		numDevices = 1
	}

	return &Device{
		bus:        bus,
		cs:         cs,
		numDevices: numDevices,
	}
}

// Configure setups the pins.
func (driver *Device) Configure(config Config) error {
	outPutConfig := machine.PinConfig{Mode: machine.PinOutput}
	driver.cs.Configure(outPutConfig)

	driver.StopDisplayTest()
	if config.NumberOfDigits < 1 || config.NumberOfDigits > 8 {
		return ErrInvalidData
	}
	driver.SetScanLimit(config.NumberOfDigits)
	if config.Intensity == 0 {
		config.Intensity = 0x01
	}
	driver.SetIntensity(config.Intensity)
	driver.SetDecodeMode(0x08) // use bcd decoding for all digits
	driver.StopShutdownMode()
	return nil
}

// SetScanLimit sets the scan limit for all devices. Maximum is 8.
// This will set the number of digits to display.
func (driver *Device) SetScanLimit(digitNumber uint8) {
	driver.writeToAll(Command{REG_SCANLIMIT, digitNumber - 1})
}

// SetIntensity sets the intensity for all displays.
// There are 16 possible intensity levels. The valid range is 0x00-0x0F
func (driver *Device) SetIntensity(intensity uint8) {
	if intensity > 0x0F {
		intensity = 0x0F
	}
	driver.writeToAll(Command{REG_INTENSITY, intensity})
}

// SetDecodeMode sets the decode mode for 7 segment displays.
// digitNumber = 1 -> 1 digit gets decoded
// digitNumber = 2 or 3, or 4 -> 4 digit are being decoded
// digitNumber = 8 -> 8 digits are being decoded
// digitNumber 0 || digitNumber > 8 -> no decoding is being used
func (driver *Device) SetDecodeMode(digitNumber uint8) {
	switch digitNumber {
	case 1: // only decode first digit
		driver.writeToAll(Command{REG_DECODE_MODE, 0x01})
	case 2, 3, 4: //  decode digits 3-0
		driver.writeToAll(Command{REG_DECODE_MODE, 0x0F})
	case 8: // decode 8 digits
		driver.writeToAll(Command{REG_DECODE_MODE, 0xFF})
	default:
		driver.writeToAll(Command{REG_DECODE_MODE, 0x00})
	}
}

// StartShutdownMode sets the IC into a low power shutdown mode.
func (driver *Device) StartShutdownMode() {
	driver.writeToAll(Command{REG_SHUTDOWN, 0x00})
}

// StartShutdownMode sets the IC into normal operation mode.
func (driver *Device) StopShutdownMode() {
	driver.writeToAll(Command{REG_SHUTDOWN, 0x01})
}

// StartDisplayTest starts a display test.
func (driver *Device) StartDisplayTest() {
	driver.writeToAll(Command{REG_DISPLAY_TEST, 0x01})
}

// StopDisplayTest stops the display test and gets into normal operation mode.
func (driver *Device) StopDisplayTest() {
	driver.writeToAll(Command{REG_DISPLAY_TEST, 0x00})
}

func (driver *Device) writeByte(data byte) {
	driver.bus.Transfer(data)
}

// WriteCommand sends a command to a specific MAX7219 in the chain
//
//	 deviceNum: 0-7 - identifies the MAX7219 device
//		data: the command to send
//
// Example: To set digit 1 to display the number 5 and DP, send the following 16-bit data:
//
//	WriteCommand(0, Command{Register: REG_DIGIT1, Data: 5 | Dot})
func (driver *Device) WriteCommand(deviceNum uint8, data Command) error {
	if deviceNum >= driver.numDevices {
		return ErrInvalidData
	}

	tmp := [maxNumberOfDevices]Command{}
	for i := range tmp {
		tmp[i] = Command{REG_NOOP, 0x00}
	}
	tmp[deviceNum] = data
	return driver.WriteCommandToAll(tmp[:driver.numDevices])
}

// WriteCommandToAll sends command to a all devices in the chain in one go.
// The data slice must have the same length as the number of devices in the chain
func (driver *Device) WriteCommandToAll(data []Command) error {
	if len(data) != int(driver.numDevices) {
		return ErrInvalidData
	}

	driver.cs.Low()
	for _, d := range data {
		driver.writeByte(d.Register)
		driver.writeByte(d.Data)
	}
	driver.cs.High()
	return nil
}

// writeToAll sends the same command to all devices in the chain
func (driver *Device) writeToAll(data Command) error {
	tmp := make([]Command, driver.numDevices)
	for i := range tmp {
		tmp[i] = data
	}
	return driver.WriteCommandToAll(tmp)
}
