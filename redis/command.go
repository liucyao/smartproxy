package redis

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/ngaut/logging"
	"gopkg.in/bufio.v1"
	"smartproxy/util"
)

var (
	_ Cmder = (*Cmd)(nil)
	_ Cmder = (*SliceCmd)(nil)
	_ Cmder = (*StatusCmd)(nil)
	_ Cmder = (*IntCmd)(nil)
	_ Cmder = (*DurationCmd)(nil)
	_ Cmder = (*BoolCmd)(nil)
	_ Cmder = (*StringCmd)(nil)
	_ Cmder = (*FloatCmd)(nil)
	_ Cmder = (*StringSliceCmd)(nil)
	_ Cmder = (*BoolSliceCmd)(nil)
	_ Cmder = (*StringStringMapCmd)(nil)
	_ Cmder = (*StringIntMapCmd)(nil)
	_ Cmder = (*ZSliceCmd)(nil)
	_ Cmder = (*ScanCmd)(nil)
	_ Cmder = (*ClusterSlotCmd)(nil)
)

type Cmder interface {
	args() []string
	parseReply(*bufio.Reader) error
	setErr(error)
	reset()

	writeTimeout() *time.Duration
	readTimeout() *time.Duration
	clusterKey() string

	Err() error
	String() string

	Reply() []byte
}

func setCmdsErr(cmds []Cmder, e error) {
	for _, cmd := range cmds {
		cmd.setErr(e)
	}
}

func resetCmds(cmds []Cmder) {
	for _, cmd := range cmds {
		cmd.reset()
	}
}

func cmdString(cmd Cmder, val interface{}) string {
	s := strings.Join(cmd.args(), " ")
	if err := cmd.Err(); err != nil {
		return s + ": " + err.Error()
	}
	if val != nil {
		return s + ": " + fmt.Sprint(val)
	}
	return s

}

//------------------------------------------------------------------------------

type baseCmd struct {
	_args []string

	err error

	_clusterKeyPos int

	_writeTimeout, _readTimeout *time.Duration
}

func (cmd *baseCmd) Err() error {
	if cmd.err != nil {
		return cmd.err
	}
	return nil
}

func (cmd *baseCmd) args() []string {
	return cmd._args
}

func (cmd *baseCmd) readTimeout() *time.Duration {
	return cmd._readTimeout
}

func (cmd *baseCmd) setReadTimeout(d time.Duration) {
	cmd._readTimeout = &d
}

func (cmd *baseCmd) writeTimeout() *time.Duration {
	return cmd._writeTimeout
}

func (cmd *baseCmd) clusterKey() string {
	if cmd._clusterKeyPos > 0 && cmd._clusterKeyPos < len(cmd._args) {
		return cmd._args[cmd._clusterKeyPos]
	}
	return ""
}

func (cmd *baseCmd) setWriteTimeout(d time.Duration) {
	cmd._writeTimeout = &d
}

func (cmd *baseCmd) setErr(e error) {
	cmd.err = e
}

//------------------------------------------------------------------------------

type Cmd struct {
	baseCmd

	val interface{}
}

func NewCmd(args ...string) *Cmd {
	return &Cmd{baseCmd: baseCmd{_args: args}}
}

func (cmd *Cmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *Cmd) Val() interface{} {
	return cmd.val
}

func (cmd *Cmd) Result() (interface{}, error) {
	return cmd.val, cmd.err
}

func (cmd *Cmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *Cmd) parseReply(rd *bufio.Reader) error {
	cmd.val, cmd.err = parseReply(rd, parseSlice)
	return cmd.err
}

func (cmd *Cmd) Reply() []byte {

	return nil
}

//------------------------------------------------------------------------------

type SliceCmd struct {
	baseCmd

	val []interface{}
}

func NewSliceCmd(args ...string) *SliceCmd {
	return &SliceCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *SliceCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *SliceCmd) Val() []interface{} {
	return cmd.val
}

func (cmd *SliceCmd) Result() ([]interface{}, error) {
	return cmd.val, cmd.err
}

func (cmd *SliceCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *SliceCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseSlice)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.([]interface{})
	return nil
}

func (cmd *SliceCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	// [nice.com 80 <nil> 1.2]
	return FormatSlice(cmd.Val())
}

func FormatSlice(val []interface{}) []byte {
	b := bytes.Buffer{}
	b.WriteByte('*')
	b.WriteString(util.Itoa(len(val)))
	b.WriteString("\r\n")
	for _, v := range val {
		if v == nil {
			b.WriteString("$-1\r\n")
			continue
		}
		switch v.(type) {
		case int:
			d := formatInt(int64(v.(int)))
			b.WriteByte('$')
			b.WriteString(util.Itoa(len(d)))
			b.WriteString("\r\n")
			b.WriteString(d)
			b.WriteString("\r\n")
		case int64:
			d := formatInt(v.(int64))
			b.WriteByte('$')
			b.WriteString(util.Itoa(len(d)))
			b.WriteString("\r\n")
			b.WriteString(d)
			b.WriteString("\r\n")
		case string:
			d := v.(string)
			b.WriteByte('$')
			b.WriteString(util.Itoa(len(d)))
			b.WriteString("\r\n")
			b.WriteString(d)
			b.WriteString("\r\n")
		case float64:
			d := formatFloat(v.(float64))
			b.WriteByte('$')
			b.WriteString(util.Itoa(len(d)))
			b.WriteString("\r\n")
			b.WriteString(d)
			b.WriteString("\r\n")
		default:
			log.Warningf("got %T , expected string or int or float ", v)
			d := fmt.Sprintf("-%s\r\n", TypeAssertedErr.Error())
			return []byte(d)
		}
	}
	return b.Bytes()

}

//------------------------------------------------------------------------------

type StatusCmd struct {
	baseCmd

	val string
}

func NewStatusCmd(args ...string) *StatusCmd {
	return &StatusCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func newKeylessStatusCmd(args ...string) *StatusCmd {
	return &StatusCmd{baseCmd: baseCmd{_args: args}}
}

func (cmd *StatusCmd) reset() {
	cmd.val = ""
	cmd.err = nil
}

func (cmd *StatusCmd) Val() string {
	return cmd.val
}

func (cmd *StatusCmd) Result() (string, error) {
	return cmd.val, cmd.err
}

func (cmd *StatusCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *StatusCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, nil)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.(string)
	return nil
}

func (cmd *StatusCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)
	}
	return FormatStatus(cmd.Val())
}

func FormatStatus(val string) []byte {
	b := bytes.Buffer{}
	b.WriteString("+")
	b.WriteString(val)
	b.WriteString("\r\n")
	return b.Bytes()
}

//------------------------------------------------------------------------------

type IntCmd struct {
	baseCmd

	val int64
}

func NewIntCmd(args ...string) *IntCmd {
	return &IntCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *IntCmd) reset() {
	cmd.val = 0
	cmd.err = nil
}

func (cmd *IntCmd) Val() int64 {
	return cmd.val
}

func (cmd *IntCmd) Result() (int64, error) {
	return cmd.val, cmd.err
}

func (cmd *IntCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *IntCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, nil)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.(int64)
	return nil
}

func (cmd *IntCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	return FormatInt(cmd.Val())
}

func FormatInt(val int64) []byte {
	b := bytes.Buffer{}
	b.WriteByte(':')
	b.WriteString(formatInt(val))
	b.WriteString("\r\n")
	return b.Bytes()
}

//------------------------------------------------------------------------------

type DurationCmd struct {
	baseCmd

	val       time.Duration
	precision time.Duration
}

func NewDurationCmd(precision time.Duration, args ...string) *DurationCmd {
	return &DurationCmd{
		precision: precision,
		baseCmd:   baseCmd{_args: args, _clusterKeyPos: 1},
	}
}

func (cmd *DurationCmd) reset() {
	cmd.val = 0
	cmd.err = nil
}

func (cmd *DurationCmd) Val() time.Duration {
	return cmd.val
}

func (cmd *DurationCmd) Result() (time.Duration, error) {
	return cmd.val, cmd.err
}

func (cmd *DurationCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *DurationCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, nil)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = time.Duration(v.(int64)) * cmd.precision
	return nil
}

func (cmd *DurationCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	return FormatDuration(cmd.Val(), cmd.precision)
}

func FormatDuration(val time.Duration, pre time.Duration) []byte {
	b := bytes.Buffer{}
	b.WriteByte(':')
	if pre == time.Millisecond {
		b.WriteString(formatMs(val))
	} else {
		b.WriteString(formatSec(val))
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

//------------------------------------------------------------------------------

type BoolCmd struct {
	baseCmd

	val bool
}

func NewBoolCmd(args ...string) *BoolCmd {
	return &BoolCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *BoolCmd) reset() {
	cmd.val = false
	cmd.err = nil
}

func (cmd *BoolCmd) Val() bool {
	return cmd.val
}

func (cmd *BoolCmd) Result() (bool, error) {
	return cmd.val, cmd.err
}

func (cmd *BoolCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *BoolCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, nil)
	// `SET key value NX` returns nil when key already exists.
	if err == Nil {
		cmd.val = false
		return nil
	}
	if err != nil {
		cmd.err = err
		return err
	}
	switch vv := v.(type) {
	case int64:
		cmd.val = vv == 1
		return nil
	case string:
		cmd.val = vv == "OK"
		return nil
	default:
		return fmt.Errorf("got %T, wanted int64 or string")
	}
}
func (cmd *BoolCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	return FormatBool(cmd.Val())
}

func FormatBool(val bool) []byte {
	b := bytes.Buffer{}
	b.WriteByte(':')
	if val {
		b.WriteByte('1')
	} else {
		b.WriteByte('0')
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

//------------------------------------------------------------------------------

type StringCmd struct {
	baseCmd

	val string
}

func NewStringCmd(args ...string) *StringCmd {
	return &StringCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *StringCmd) reset() {
	cmd.val = ""
	cmd.err = nil
}

func (cmd *StringCmd) Val() string {
	return cmd.val
}

func (cmd *StringCmd) Result() (string, error) {
	return cmd.val, cmd.err
}

func (cmd *StringCmd) Int64() (int64, error) {
	if cmd.err != nil {
		return 0, cmd.err
	}
	return strconv.ParseInt(cmd.val, 10, 64)
}

func (cmd *StringCmd) Uint64() (uint64, error) {
	if cmd.err != nil {
		return 0, cmd.err
	}
	return strconv.ParseUint(cmd.val, 10, 64)
}

func (cmd *StringCmd) Float64() (float64, error) {
	if cmd.err != nil {
		return 0, cmd.err
	}
	return strconv.ParseFloat(cmd.val, 64)
}

func (cmd *StringCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *StringCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, nil)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.(string)
	return nil
}

func (cmd *StringCmd) Reply() []byte {

	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	return FormatString(cmd.Val())
}

func FormatString(val string) []byte {
	b := bytes.Buffer{}
	b.WriteByte('$')
	b.WriteString(util.Itoa(len(val)))
	b.WriteString("\r\n")
	b.WriteString(val)
	b.WriteString("\r\n")
	return b.Bytes()
}

//------------------------------------------------------------------------------

type FloatCmd struct {
	baseCmd

	val float64
}

func NewFloatCmd(args ...string) *FloatCmd {
	return &FloatCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *FloatCmd) reset() {
	cmd.val = 0
	cmd.err = nil
}

func (cmd *FloatCmd) Val() float64 {
	return cmd.val
}

func (cmd *FloatCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *FloatCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, nil)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val, cmd.err = strconv.ParseFloat(v.(string), 64)
	return cmd.err
}
func (cmd *FloatCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	return FormatFloat(cmd.Val())
}

func FormatFloat(val float64) []byte {
	b := bytes.Buffer{}
	b.WriteByte('$')
	d := formatFloat(val)
	b.WriteString(util.Itoa(len(d)))
	b.WriteString("\r\n")
	b.WriteString(d)
	b.WriteString("\r\n")
	return b.Bytes()
}

//------------------------------------------------------------------------------

type StringSliceCmd struct {
	baseCmd

	val []string
}

func NewStringSliceCmd(args ...string) *StringSliceCmd {
	return &StringSliceCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *StringSliceCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *StringSliceCmd) Val() []string {
	return cmd.val
}

func (cmd *StringSliceCmd) Result() ([]string, error) {
	return cmd.Val(), cmd.Err()
}

func (cmd *StringSliceCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *StringSliceCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseStringSlice)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.([]string)
	return nil
}

func (cmd *StringSliceCmd) Reply() []byte {
	err := cmd.Err()

	if err != nil {
		if err.Error() == "redis: nil" {
			return []byte("$-1\r\n")
		}
		d := fmt.Sprintf("-%s\r\n", err.Error())
		return []byte(d)

	}
	return FormatStringSlice(cmd.Val())
}

func FormatStringSlice(val []string) []byte {
	b := bytes.Buffer{}
	b.WriteByte('*')
	b.WriteString(util.Itoa(len(val)))
	b.WriteString("\r\n")
	for _, v := range val {
		b.WriteByte('$')
		b.WriteString(util.Itoa(len(v)))
		b.WriteString("\r\n")
		b.WriteString(v)
		b.WriteString("\r\n")
	}
	return b.Bytes()
}

//------------------------------------------------------------------------------

type BoolSliceCmd struct {
	baseCmd

	val []bool
}

func NewBoolSliceCmd(args ...string) *BoolSliceCmd {
	return &BoolSliceCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *BoolSliceCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *BoolSliceCmd) Val() []bool {
	return cmd.val
}

func (cmd *BoolSliceCmd) Result() ([]bool, error) {
	return cmd.val, cmd.err
}

func (cmd *BoolSliceCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *BoolSliceCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseBoolSlice)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.([]bool)
	return nil
}

func (cmd *BoolSliceCmd) Reply() []byte {

	return nil
}

//------------------------------------------------------------------------------

type StringStringMapCmd struct {
	baseCmd

	val map[string]string
}

func NewStringStringMapCmd(args ...string) *StringStringMapCmd {
	return &StringStringMapCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *StringStringMapCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *StringStringMapCmd) Val() map[string]string {
	return cmd.val
}

func (cmd *StringStringMapCmd) Result() (map[string]string, error) {
	return cmd.val, cmd.err
}

func (cmd *StringStringMapCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *StringStringMapCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseStringStringMap)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.(map[string]string)
	return nil
}

func (cmd *StringStringMapCmd) Reply() []byte {

	return nil
}

//------------------------------------------------------------------------------

type StringIntMapCmd struct {
	baseCmd

	val map[string]int64
}

func NewStringIntMapCmd(args ...string) *StringIntMapCmd {
	return &StringIntMapCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *StringIntMapCmd) Val() map[string]int64 {
	return cmd.val
}

func (cmd *StringIntMapCmd) Result() (map[string]int64, error) {
	return cmd.val, cmd.err
}

func (cmd *StringIntMapCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *StringIntMapCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *StringIntMapCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseStringIntMap)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.(map[string]int64)
	return nil
}
func (cmd *StringIntMapCmd) Reply() []byte {

	return nil
}

//------------------------------------------------------------------------------

type ZSliceCmd struct {
	baseCmd

	val []Z
}

func NewZSliceCmd(args ...string) *ZSliceCmd {
	return &ZSliceCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *ZSliceCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *ZSliceCmd) Val() []Z {
	return cmd.val
}

func (cmd *ZSliceCmd) Result() ([]Z, error) {
	return cmd.val, cmd.err
}

func (cmd *ZSliceCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *ZSliceCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseZSlice)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.([]Z)
	return nil
}
func (cmd *ZSliceCmd) Reply() []byte {

	return nil
}

//------------------------------------------------------------------------------

type ScanCmd struct {
	baseCmd

	cursor int64
	keys   []string
}

func NewScanCmd(args ...string) *ScanCmd {
	return &ScanCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *ScanCmd) reset() {
	cmd.cursor = 0
	cmd.keys = nil
	cmd.err = nil
}

func (cmd *ScanCmd) Val() (int64, []string) {
	return cmd.cursor, cmd.keys
}

func (cmd *ScanCmd) Result() (int64, []string, error) {
	return cmd.cursor, cmd.keys, cmd.err
}

func (cmd *ScanCmd) String() string {
	return cmdString(cmd, cmd.keys)
}

func (cmd *ScanCmd) parseReply(rd *bufio.Reader) error {
	vi, err := parseReply(rd, parseSlice)
	if err != nil {
		cmd.err = err
		return cmd.err
	}
	v := vi.([]interface{})

	cmd.cursor, cmd.err = strconv.ParseInt(v[0].(string), 10, 64)
	if cmd.err != nil {
		return cmd.err
	}

	keys := v[1].([]interface{})
	for _, keyi := range keys {
		cmd.keys = append(cmd.keys, keyi.(string))
	}

	return nil
}

func (cmd *ScanCmd) Reply() []byte {

	return nil
}

//------------------------------------------------------------------------------

type ClusterSlotInfo struct {
	Start, End int
	Addrs      []string
}

type ClusterSlotCmd struct {
	baseCmd

	val []ClusterSlotInfo
}

func NewClusterSlotCmd(args ...string) *ClusterSlotCmd {
	return &ClusterSlotCmd{baseCmd: baseCmd{_args: args, _clusterKeyPos: 1}}
}

func (cmd *ClusterSlotCmd) Val() []ClusterSlotInfo {
	return cmd.val
}

func (cmd *ClusterSlotCmd) Result() ([]ClusterSlotInfo, error) {
	return cmd.Val(), cmd.Err()
}

func (cmd *ClusterSlotCmd) String() string {
	return cmdString(cmd, cmd.val)
}

func (cmd *ClusterSlotCmd) reset() {
	cmd.val = nil
	cmd.err = nil
}

func (cmd *ClusterSlotCmd) parseReply(rd *bufio.Reader) error {
	v, err := parseReply(rd, parseClusterSlotInfoSlice)
	if err != nil {
		cmd.err = err
		return err
	}
	cmd.val = v.([]ClusterSlotInfo)
	return nil
}

func (cmd *ClusterSlotCmd) Reply() []byte {

	return nil
}
