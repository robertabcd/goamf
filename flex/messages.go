package flex

import (
	"amf"
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
		if b&0x80 == 0x80 {
			break
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

type UUID string

func readAndCheckError(d *amf.Decoder, flag uint8, args ...interface{}) (parsed int, err error) {
	for i := 0; i < len(args); i += 2 {
		f := args[i].(uint8)
		if flag&f != f {
			continue
		}
		if err = d.ReadValue(args[i+1]); err != nil {
			return
		}
		parsed++
	}
	return
}

/* Registration */
func RegisterToTraitsMapper(mapper *amf.TraitsMapper) {
	mapper.RegisterType(AsyncMessageExt{}, &amf.Traits{
		ClassName: "DSA",
		External:  true,
	})
	mapper.RegisterType(AsyncMessageExt{}, &amf.Traits{
		ClassName: "flex.messaging.messages.AsyncMessageExt",
		External:  true,
	})

	mapper.RegisterType(AcknowledgeMessageExt{}, &amf.Traits{
		ClassName: "DSK",
		External:  true,
	})
	mapper.RegisterType(AcknowledgeMessageExt{}, &amf.Traits{
		ClassName: "flex.messaging.messages.AcknowledgeMessageExt",
		External:  true,
	})

	mapper.RegisterType(CommandMessageExt{}, &amf.Traits{
		ClassName: "DSC",
		External:  true,
	})
	mapper.RegisterType(CommandMessageExt{}, &amf.Traits{
		ClassName: "flex.messaging.messages.CommandMessageExt",
		External:  true,
	})

	mapper.RegisterType(RemotingMessage{}, &amf.Traits{
		ClassName: "flex.messaging.messages.RemotingMessage",
		// TODO: add members
	})
}

func init() {
	RegisterToTraitsMapper(amf.DefaultTraitsMapper)
}

/* AbstractMessage */
type AbstractMessage struct {
	Body        interface{}
	CliendId    UUID
	Destination string
	Headers     interface{}
	MessageId   UUID
	Timestamp   string
	TimeToLive  int64

	flags Flags
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

func (m *AbstractMessage) ReadExternal(d *amf.Decoder) error {
	if err := m.flags.ReadExternal(d); err != nil {
		return err
	}

	totalParsed := 0
	nflags := m.flags.Len()
	if nflags > 0 {
		if parsed, err := readAndCheckError(d, m.flags.At(0),
			AbstractMessage_Body, &m.Body,
			AbstractMessage_ClientId, &m.CliendId,
			AbstractMessage_Destination, &m.Destination,
			AbstractMessage_Headers, &m.Headers,
			AbstractMessage_MessageId, &m.MessageId,
			AbstractMessage_Timestamp, &m.Timestamp,
			AbstractMessage_TimeToLive, &m.TimeToLive); err != nil {
			return err
		} else {
			totalParsed += parsed
		}
	}
	if nflags > 1 {
		// TODO: convert it to UUID
		var dummy0, dummy1 interface{}
		if parsed, err := readAndCheckError(d, m.flags.At(1),
			AbstractMessage_ClientIdBytes, &dummy0,
			AbstractMessage_MessageIdBytes, &dummy1); err != nil {
			return err
		} else {
			totalParsed += parsed
		}
	}
	for i := m.flags.CountBits() - totalParsed; i > 0; i-- {
		var dummy interface{}
		if err := d.ReadValue(&dummy); err != nil {
			return err
		}
	}

	return nil
}

/* AsyncMessage */
type AsyncMessage struct {
	AbstractMessage
	CorrelationId UUID

	flags Flags
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

	totalParsed := 0
	nflags := m.flags.Len()
	if nflags > 0 {
		var dummy0 interface{}
		if parsed, err := readAndCheckError(d, m.flags.At(0),
			AsyncMessage_CorrelationId, &m.CorrelationId,
			AsyncMessage_CorrelationIdBytes, &dummy0); err != nil { // TODO: convert it to UUID
			return err
		} else {
			totalParsed += parsed
		}
	}
	for i := m.flags.CountBits() - totalParsed; i > 0; i-- {
		var dummy interface{}
		if err := d.ReadValue(&dummy); err != nil {
			return err
		}
	}
	return nil
}

/* DSA, flex.messaging.messages.AsyncMessageExt */
type AsyncMessageExt struct {
	AsyncMessage
}

func (m *AsyncMessageExt) ReadExternal(d *amf.Decoder) error {
	return m.AsyncMessage.ReadExternal(d)
}

/* AcknowledgeMessage */
type AcknowledgeMessage struct {
	AsyncMessage

	flags Flags
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

/* DSK, flex.messaging.messages.AcknowledgeMessageEx */
type AcknowledgeMessageExt struct {
	AcknowledgeMessage
}

func (m *AcknowledgeMessageExt) ReadExternal(d *amf.Decoder) error {
	return m.AcknowledgeMessage.ReadExternal(d)
}

type ErrorMessage struct {
	AcknowledgeMessage
}

func (m *ErrorMessage) ReadExternal(d *amf.Decoder) error {
	return m.AcknowledgeMessage.ReadExternal(d)
}

/* CommandMessage */
type CommandMessage struct {
	AsyncMessage
	Operation string

	flags Flags
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

	totalParsed := 0
	nflags := m.flags.Len()
	if nflags > 0 {
		if parsed, err := readAndCheckError(d, m.flags.At(0),
			CommandMessage_Operation, &m.Operation); err != nil {
			return err
		} else {
			totalParsed += parsed
		}
	}
	for i := m.flags.CountBits() - totalParsed; i > 0; i-- {
		var dummy interface{}
		if err := d.ReadValue(&dummy); err != nil {
			return err
		}
	}
	return nil
}

/* DSC, flex.messaging.messages.CommandMessageExt */
type CommandMessageExt struct {
	CommandMessage
}

func (m *CommandMessageExt) ReadExternal(d *amf.Decoder) error {
	return m.CommandMessage.ReadExternal(d)
}

/* flex.messaging.messages.RemotingMessage */
type RemotingMessage struct {
	Source      interface{}
	Operation   string
	ClientId    UUID
	Headers     interface{}
	Body        interface{}
	Timestamp   int64
	TimeToLive  int64
	Destination string
	MessageId   UUID
}
