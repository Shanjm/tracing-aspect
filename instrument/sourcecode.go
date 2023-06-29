package instrument

const SourceCode = `// Package instrument
// MultiMode，支持并行模式，且尽量少的干扰源代码逻辑的方案
package instrument

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"reflect"
	"sync"
	"time"

	"github.com/petermattis/goid"
)

// 包内锁
var lock sync.Mutex

// 并行方案
type tracer struct {
	id        int64           // goroutine ID
	req       string          // 请求
	rsp       string          // 响应
	funcInput string          // 函数输入
	funcOuput string          // 函数输出
	children  sync.Map        // 子调用
	root      *tracer         // 指向根trace
	wg        *sync.WaitGroup // 等待子调用结束
}

func (t *tracer) print() {
	fmt.Printf("goid: %d\n", t.id)
	if t.req != "" {
		fmt.Println("请求：", t.req)
	}
	if t.rsp != "" {
		fmt.Println("返回：", t.rsp)
	}
	fmt.Printf("输入：\n%s", t.funcInput)
	fmt.Printf("输出：\n%s", t.funcOuput)

	t.children.Range(func(key, value interface{}) bool {
		fmt.Printf("goid: %d 开辟了子协程 goid: %v\n", t.id, key)
		tr := value.(*tracer)
		tr.print()
		return true
	})
}

// 根 Trace
var TracerManager sync.Map

// StartMultiMode 开始并行模式
func StartMultiMode() {
	id := goid.Get()
	t := &tracer{
		id:       id,
		children: sync.Map{},
		wg:       &sync.WaitGroup{},
		root:     nil,
	}
	t.root = t
	TracerManager.Store(id, t)
}

// 关闭子协程
func CloseGoRoutine() {
	id := goid.Get()
	t, ok := TracerManager.Load(id)
	if !ok {
		fmt.Printf("标识关闭的线程失败: %d\n", id)
	}

	tr := t.(*tracer)
	tr.wg.Wait() // 至少等待子协程注册完成
	TracerManager.Delete(id)
	tr.root.wg.Done() // 让根 tracer 减1
}

// 注册子协程
func RegisterChildrenId(pid int64) {
	cid := goid.Get()
	ct := &tracer{
		id:       cid,
		children: sync.Map{},
		wg:       &sync.WaitGroup{},
		root:     nil,
	}

	if trace, ok := TracerManager.Load(pid); ok {
		if t, ok := trace.(*tracer); ok {
			ct.root = t.root
			t.children.Store(cid, ct)
			// 注册完成后，让父 tracer 减1
			t.wg.Done()
			// 只有找到父 tarcer 才能存入 TM
			TracerManager.Store(cid, ct)
		}
	}
}

// IncreaseWG waitgroup 自增
func IncreaseWG() {
	id := goid.Get()
	if trace, ok := TracerManager.Load(id); ok {
		if t, ok := trace.(*tracer); ok {
			t.root.wg.Add(1) // 让根 tracer 加1
			t.wg.Add(1)      // 本身也加1
		}
	}
}

// StopMultiMode 结束记录
func StopMultiMode() {
	id := goid.Get()
	t, ok := TracerManager.Load(id)
	if !ok {
		fmt.Printf("id: %d 不存在map中\n", id)
		return
	}
	rootTracer, ok := t.(*tracer)
	if !ok {
		fmt.Printf("id: %d 不能转换为*tracer\n", id)
	}

	isDone := make(chan struct{})
	go func() {
		rootTracer.wg.Wait()
		close(isDone)
	}()

	select {
	case <-time.After(30 * time.Second): // 30s 超时防止子协程卡住
		fmt.Println("超时退出root tracer")
	case <-isDone:
		fmt.Println("正常退出root tracer")
	}

	TracerManager.Delete(id)

	// todo
	// 锁住，进行打印
	lock.Lock()
	defer lock.Unlock()
	fmt.Println("-----------------START-----------------")
	// 输出所有的 output
	rootTracer.print()
	fmt.Println("------------------END------------------")
}

// ReportInput 记录切面数据
func ReportInput(args ...interface{}) {
	record := Report(args...)
	gid := goid.Get()
	if trace, ok := TracerManager.Load(gid); ok {
		if t, ok := trace.(*tracer); ok {
			t.funcInput = t.funcInput + record
		}
	}
}

// ReportOutput 记录切面数据
func ReportOutput(args ...interface{}) {
	record := Report(args...)
	gid := goid.Get()
	if trace, ok := TracerManager.Load(gid); ok {
		if t, ok := trace.(*tracer); ok {
			t.funcOuput = t.funcOuput + record
		}
	}
}

// Report 上报切面数据
func Report(args ...interface{}) string {
	ret := ""
	for _, arg := range args {
		arg = convert(arg)
		ret = ret + "|" + fmt.Sprint(arg)
	}
	return ret + "\n"
}

// 转换成 string 类型的数据
func convert(value interface{}) string {
	if value == nil {
		return "nil"
	}
	switch reflect.TypeOf(value).Kind() {
	case reflect.Invalid:
		return "该参数类型为Invalid"
	case reflect.Func:
		return "该参数类型为Func"
	case reflect.Chan:
		return "该参数类型为Channel"
	case reflect.UnsafePointer:
		return "该参数类型为UnsafePointer"
	case reflect.Ptr:
		rv := reflect.ValueOf(value).Elem()
		if rv.Kind() == reflect.Invalid {
			return "nil"
		}
		if rv.CanInterface() {
			return convert(rv.Interface())
		}
		return fmt.Sprint(rv)
	case reflect.Array, reflect.Slice:
		retV := []interface{}{}
		isByte := true
		arrayValue := reflect.ValueOf(value)
		for i := 0; i < arrayValue.Len(); i++ {
			va := arrayValue.Index(i)
			if va.Kind() != reflect.Uint8 {
				isByte = false
			}
			if va.CanInterface() {
				retV = append(retV, va.Interface())
			} else {
				retV = append(retV, fmt.Sprint(va))
			}
		}
		if isByte {
			b := []byte{}
			for _, bv := range retV {
				b = append(b, bv.(byte))
			}
			return string(b)
		}
		return fmt.Sprint(retV)
	case reflect.Map:
		mv := "map["
		it := reflect.ValueOf(value).MapRange()
		for it.Next() {
			k, v := it.Key(), it.Value()
			key, vv := fmt.Sprint(k), fmt.Sprint(v)
			if k.CanInterface() {
				key = convert(k.Interface())
			}
			if v.CanInterface() {
				vv = convert(v.Interface())
			}
			mv += fmt.Sprintf("%v=%v,", key, vv)
		}
		return mv + "]"
	case reflect.Struct:
		sv := reflect.ValueOf(value)
		svString := "{"
		for i := 0; i < sv.NumField(); i++ {
			member, actualValue := sv.Field(i), ""
			if member.CanInterface() {
				actualValue = convert(member.Interface())
			} else {
				actualValue = fmt.Sprint(actualValue)
			}
			svString += actualValue + " "
		}
		return svString + "}"
	default:
		return fmt.Sprintf("%v", value)
	}
}

// Origin Http
type HttpRecorder struct {
	W          http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
}

func NewRecorder(w http.ResponseWriter) *HttpRecorder {
	return &HttpRecorder{
		W:          w,
		StatusCode: http.StatusOK,
		Body:       new(bytes.Buffer),
	}
}

func (rw *HttpRecorder) WriteHeader(statusCode int) {
	rw.StatusCode = statusCode
	rw.W.WriteHeader(statusCode)
}

func (rw *HttpRecorder) Write(buf []byte) (int, error) {
	_, _ = rw.Body.Write(buf)
	return rw.W.Write(buf)
}

func (rw *HttpRecorder) Header() http.Header {
	return rw.W.Header()
}

func DumpOriHttp(req *http.Request, rw http.ResponseWriter) {
	id := goid.Get()
	t, ok := TracerManager.Load(id)
	if !ok {
		return
	}

	tracer := t.(*tracer)
	reqB, _ := httputil.DumpRequest(req, true)
	// rspB, _ := httputil.DumpResponse(rw., true)
	tracer.req = string(reqB)
	tracer.rsp = rw.(*HttpRecorder).Body.String()
}

`
