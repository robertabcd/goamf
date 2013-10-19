package flex

import (
	"amf"
	"fmt"
	"math/rand"
	"reflect"
)

type Flags struct {
	flags []uint8
}

func (f *Flags) ReadExternal(d *amf.Decoder) error {
	for {
		b, err := d.ReadUInt8()
		if err != nil {
			return err
		}
		f.flags = append(f.flags, b&0x7F)
		if b&0x80 != 0x80 {
			break
		}
	}
	return nil
}

func (f *Flags) WriteExternal(e *amf.Encoder) error {
	for i, b := range f.flags {
		if i < len(f.flags)-1 {
			b |= 0x80
		}
		if err := e.WriteUInt8(b); err != nil {
			return err
		}
	}
	return nil
}

func (f *Flags) Len() int {
	return len(f.flags)
}

func (f *Flags) At(i int) uint8 {
	return f.flags[i]
}

func (f *Flags) CountBits() (count int) {
	for i := 0; i < len(f.flags); i++ {
		v := f.flags[i]
		for ; v > 0; v = v & (v - 1) {
			count++
		}
	}
	return
}

func (f *Flags) Init(length int) {
	f.flags = make([]uint8, length)
}

func (f *Flags) Set(i int, bitmask uint8) {
	f.flags[i] |= bitmask
}

type UUID string

func NewUUID() *UUID {
	uuid := UUID(fmt.Sprintf("%08X-%04X-%04X-%04X-%04X%08X",
		rand.Uint32(),
		rand.Uint32()&0xFFFF,
		rand.Uint32()&0xFFFF,
		rand.Uint32()&0xFFFF,
		rand.Uint32()&0xFFFF,
		rand.Uint32()))
	return &uuid
}

func readAndCheckError(d *amf.Decoder, flag uint8, args ...interface{}) error {
	for i := 0; i < len(args); i += 2 {
		f := args[i].(uint8)
		if flag&f != f {
			continue
		}
		if err := d.ReadValue(args[i+1]); err != nil {
			return err
		}
	}
	for remaining := flag >> uint(len(args)/2); remaining > 0; remaining >>= 1 {
		if remaining&1 == 1 {
			var dummy interface{}
			if err := d.ReadValue(&dummy); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeAndCheckError(e *amf.Encoder, flag uint8, args ...interface{}) error {
	for i := 0; i < len(args); i += 2 {
		f := args[i].(uint8)
		if flag&f != f {
			continue
		}
		if err := e.WriteValue(args[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func setFlags(idx int, fl *Flags, args ...interface{}) {
	for i := 0; i < len(args); i += 2 {
		v := reflect.ValueOf(args[i+1])
		for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
			v = v.Elem()
		}
		if v.IsValid() {
			fl.Set(idx, args[i].(uint8))
		}
	}
}

/* Registration */
func RegisterToTraitsMapper(mapper *amf.TraitsMapper) {
	mapper.RegisterType(AsyncMessageExt{}, &amf.Traits{
		ClassName: "DSA",
		External:  true,
	})
	mapper.RegisterType(AcknowledgeMessageExt{}, &amf.Traits{
		ClassName: "DSK",
		External:  true,
	})
	mapper.RegisterType(CommandMessageExt{}, &amf.Traits{
		ClassName: "DSC",
		External:  true,
	})
	mapper.RegisterType(AsyncMessage{},
		amf.NewTraits(AsyncMessage{}, "flex.messaging.messages.AsyncMessage", false))
	mapper.RegisterType(AcknowledgeMessage{},
		amf.NewTraits(AcknowledgeMessage{}, "flex.messaging.messages.AcknowledgeMessage", false))
	mapper.RegisterType(CommandMessage{},
		amf.NewTraits(CommandMessage{}, "flex.messaging.messages.CommandMessage", false))
	mapper.RegisterType(ErrorMessage{},
		amf.NewTraits(ErrorMessage{}, "flex.messaging.messages.ErrorMessage", false))
	mapper.RegisterType(RemotingMessage{},
		amf.NewTraits(RemotingMessage{}, "flex.messaging.messages.RemotingMessage", false))
}

func init() {
	RegisterToTraitsMapper(amf.DefaultTraitsMapper)
}

/* AbstractMessage */
type AbstractMessage struct {
	Body        interface{} `amf3:"body"`
	CliendId    *UUID       `amf3:"clientId"`
	Destination *string     `amf3:"destination"`
	Headers     interface{} `amf3:"headers"`
	MessageId   *UUID       `amf3:"messageId"`
	Timestamp   *int64      `amf3:"timestamp"`
	TimeToLive  *int64      `amf3:"timeToLive"`

	flags Flags
}

func (m *AbstractMessage) GetAbstractMessage() *AbstractMessage {
	return m
}

// Flag byte 1
const (
	AbstractMessage_Body uint8 = 1 << iota
	AbstractMessage_ClientId
	AbstractMessage_Destination
	AbstractMessage_Headers
	AbstractMessage_MessageId
	AbstractMessage_Timestamp
	AbstractMessage_TimeToLive
)

// Flag byte 2
const (
	AbstractMessage_ClientIdBytes uint8 = 1 << iota
	AbstractMessage_MessageIdBytes
)

func (m *AbstractMessage) flagData0() []interface{} {
	return []interface{}{
		AbstractMessage_Body, &m.Body,
		AbstractMessage_ClientId, &m.CliendId,
		AbstractMessage_Destination, &m.Destination,
		AbstractMessage_Headers, &m.Headers,
		AbstractMessage_MessageId, &m.MessageId,
		AbstractMessage_Timestamp, &m.Timestamp,
		AbstractMessage_TimeToLive, &m.TimeToLive,
	}
}

func (m *AbstractMessage) ReadExternal(d *amf.Decoder) error {
	if err := m.flags.ReadExternal(d); err != nil {
		return err
	}

	nflags := m.flags.Len()
	if nflags > 0 {
		if err := readAndCheckError(d, m.flags.At(0), m.flagData0()...); err != nil {
			return err
		}
	}
	if nflags > 1 {
		// TODO: convert it to UUID
		var dummy0, dummy1 interface{}
		if err := readAndCheckError(d, m.flags.At(1),
			AbstractMessage_ClientIdBytes, &dummy0,
			AbstractMessage_MessageIdBytes, &dummy1); err != nil {
			return err
		}
	}

	return nil
}

func (m *AbstractMessage) WriteExternal(e *amf.Encoder) error {
	fd0 := m.flagData0()
	m.flags.Init(2)
	setFlags(0, &m.flags, fd0...)
	if err := m.flags.WriteExternal(e); err != nil {
		return err
	}
	return writeAndCheckError(e, m.flags.At(0), fd0...)
}

/* AsyncMessage */
type AsyncMessage struct {
	AbstractMessage
	CorrelationId *UUID `amf3:"correlationId"`

	flags Flags
}

func (m *AsyncMessage) GetAsyncMessage() *AsyncMessage {
	return m
}

// Flag byte 1
const (
	AsyncMessage_CorrelationId uint8 = 1 << iota
	AsyncMessage_CorrelationIdBytes
)

func (m *AsyncMessage) ReadExternal(d *amf.Decoder) error {
	if err := m.AbstractMessage.ReadExternal(d); err != nil {
		return err
	}
	if err := m.flags.ReadExternal(d); err != nil {
		return err
	}

	nflags := m.flags.Len()
	if nflags > 0 {
		var dummy0 interface{}
		if err := readAndCheckError(d, m.flags.At(0),
			AsyncMessage_CorrelationId, &m.CorrelationId,
			AsyncMessage_CorrelationIdBytes, &dummy0); err != nil { // TODO: convert it to UUID
			return err
		}
	}
	return nil
}

func (m *AsyncMessage) WriteExternal(e *amf.Encoder) error {
	if err := m.AbstractMessage.WriteExternal(e); err != nil {
		return err
	}
	m.flags.Init(1)
	m.flags.Set(0, AsyncMessage_CorrelationId)
	if err := m.flags.WriteExternal(e); err != nil {
		return err
	}
	return writeAndCheckError(e, m.flags.At(0), AsyncMessage_CorrelationId, m.CorrelationId)
}

/* DSA, flex.messaging.messages.AsyncMessageExt */
type AsyncMessageExt struct {
	AsyncMessage
}

func (m *AsyncMessageExt) ReadExternal(d *amf.Decoder) error {
	return m.AsyncMessage.ReadExternal(d)
}

func (m *AsyncMessageExt) WriteExternal(e *amf.Encoder) error {
	return m.AsyncMessage.WriteExternal(e)
}

/* AcknowledgeMessage */
type AcknowledgeMessage struct {
	AsyncMessage

	flags Flags
}

func (m *AcknowledgeMessage) GetAcknowledgeMessage() *AcknowledgeMessage {
	return m
}

func (m *AcknowledgeMessage) ReadExternal(d *amf.Decoder) error {
	if err := m.AsyncMessage.ReadExternal(d); err != nil {
		return err
	}
	if err := m.flags.ReadExternal(d); err != nil {
		return err
	}
	// No flags defined
	for i := m.flags.CountBits(); i > 0; i-- {
		var dummy interface{}
		if err := d.ReadValue(&dummy); err != nil {
			return err
		}
	}
	return nil
}

func (m *AcknowledgeMessage) WriteExternal(e *amf.Encoder) error {
	if err := m.AsyncMessage.WriteExternal(e); err != nil {
		return nil
	}
	m.flags.Init(1)
	return m.flags.WriteExternal(e)
}

/* DSK, flex.messaging.messages.AcknowledgeMessageExt */
type AcknowledgeMessageExt struct {
	AcknowledgeMessage
}

func (m *AcknowledgeMessageExt) ReadExternal(d *amf.Decoder) error {
	return m.AcknowledgeMessage.ReadExternal(d)
}

func (m *AcknowledgeMessageExt) WriteExternal(e *amf.Encoder) error {
	return m.AcknowledgeMessage.WriteExternal(e)
}

type ErrorMessage struct {
	AcknowledgeMessage
	FaultCode    *string     `amf3:"faultCode"`
	FaultDetail  *string     `amf3:"faultDetail"`
	FaultString  *string     `amf3:"faultString"`
	RootCause    interface{} `amf3:"rootCause"`
	ExtendedData interface{} `amf3:"extendedData"`
}

func (m *ErrorMessage) GetErrorMessage() *ErrorMessage {
	return m
}

/* CommandMessage */
type CommandMessage struct {
	AsyncMessage
	Operation *int32 `amf3:"operation"`

	flags Flags
}

func (m *CommandMessage) GetCommandMessage() *CommandMessage {
	return m
}

// Flag byte 1
const (
	CommandMessage_Operation uint8 = 1 << iota
)

func (m *CommandMessage) ReadExternal(d *amf.Decoder) error {
	if err := m.AsyncMessage.ReadExternal(d); err != nil {
		return err
	}
	if err := m.flags.ReadExternal(d); err != nil {
		return err
	}

	nflags := m.flags.Len()
	if nflags > 0 {
		if err := readAndCheckError(d, m.flags.At(0),
			CommandMessage_Operation, &m.Operation); err != nil {
			return err
		}
	}
	return nil
}

func (m *CommandMessage) WriteExternal(e *amf.Encoder) error {
	if err := m.AsyncMessage.WriteExternal(e); err != nil {
		return err
	}
	m.flags.Init(1)
	m.flags.Set(0, CommandMessage_Operation)
	if err := m.flags.WriteExternal(e); err != nil {
		return err
	}
	return writeAndCheckError(e, m.flags.At(0), CommandMessage_Operation, m.Operation)
}

/* DSC, flex.messaging.messages.CommandMessageExt */
type CommandMessageExt struct {
	CommandMessage
}

func (m *CommandMessageExt) ReadExternal(d *amf.Decoder) error {
	return m.CommandMessage.ReadExternal(d)
}

func (m *CommandMessageExt) WriteExternal(e *amf.Encoder) error {
	return m.CommandMessage.WriteExternal(e)
}

/* flex.messaging.messages.RemotingMessage */
type RemotingMessage struct {
	AbstractMessage
	Source    interface{} `amf3:"source"`
	Operation *string     `amf3:"operation"`
}

func (m *RemotingMessage) GetRemotingMessage() *RemotingMessage {
	return m
}
