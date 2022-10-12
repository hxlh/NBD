package nbdserver

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

type NBDServer struct {
	listener   net.Listener
	clients    []*client
	quit       uint32
	exportName string // name of the export as selected by nbd-client
	exportSize uint64
	fd         *os.File
}

type client struct {
	conn   net.Conn
	status ConnStatus
}

type nbdRequest struct {
	magic   uint32
	reqType uint32
	handle  uint64
	from    uint64
	size    uint32
}

func (req *nbdRequest) Write(w io.Writer) error {
	err := binary.Write(w, binary.BigEndian, req.magic)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, req.reqType)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, req.handle)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, req.from)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, req.size)
	if err != nil {
		return err
	}
	return nil
}

func (req *nbdRequest) Read(r io.Reader) error {
	err := binary.Read(r, binary.BigEndian, &req.magic)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.BigEndian, &req.reqType)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.BigEndian, &req.handle)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.BigEndian, &req.from)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.BigEndian, &req.size)
	if err != nil {
		return err
	}
	return nil
}

type nbdReply struct {
	magic  uint32
	err    uint32
	handle uint64
}

func (reply *nbdReply) Write(w io.Writer) error {
	err := binary.Write(w, binary.BigEndian, reply.magic)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, reply.err)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, reply.handle)
	if err != nil {
		return err
	}
	return nil
}

func (reply *nbdReply) Read(r io.Reader) error {
	err := binary.Read(r, binary.BigEndian, &reply.magic)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.BigEndian, &reply.err)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.BigEndian, &reply.handle)
	if err != nil {
		return err
	}
	return nil
}

type ConnStatus int

const (
	NBD_HandShake ConnStatus = iota
	NBD_Transmission
)

const _kb = 1024
const _mb = 1024 * _kb
const _gb = 1024 * _mb

func New() *NBDServer {
	st := time.Now()
	srv := &NBDServer{
		quit:       0,
		exportName: "export",
		exportSize: 512 * _mb,
	}
	_, err := os.Stat("block_file")
	if err != nil {
		var err error
		srv.fd, err = os.OpenFile("block_file", os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatalf("create block file err : %v\n", err)
		}
		bufSize := 4 * _mb
		buf := make([]byte, bufSize)
		for i := 0; i < int(srv.exportSize); i += bufSize {
			_, err := srv.fd.Write(buf)
			if err != nil {
				log.Printf("nbd server init block file failed\n")
			}
		}
	} else {
		var err error
		srv.fd, err = os.OpenFile("block_file", os.O_RDWR, 0644)
		if err != nil {
			log.Fatalf("open block file err : %v\n", err)
		}
	}
	log.Printf("nbd server init block file success; used: %v\n", time.Since(st).String())
	return srv
}

func (srv *NBDServer) RunWithLoop(addr string) {
	var err error
	srv.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return
	}

	// 处理退出
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill)
	go srv.handleQuit(sigs)

	for atomic.LoadUint32(&srv.quit) == 0 {
		c, err := srv.listener.Accept()
		if err != nil {
			continue
		}
		if conn, ok := c.(*net.TCPConn); ok {
			conn.SetNoDelay(true)
		}
		client := &client{
			conn:   c,
			status: NBD_HandShake,
		}
		log.Printf("client %v connecting \n", c.RemoteAddr().String())
		srv.clients = append(srv.clients, client)
		go srv.handleLoop(client)
	}

	signal.Stop(sigs)
}

func (srv *NBDServer) handleLoop(c *client) {
	defer c.conn.Close()
	for {
		switch c.status {
		case NBD_HandShake:
			if err := srv.negotiate(c); err != nil {
				log.Printf("Negotiate Err: %s\n", err.Error())
				return
			}
			// 握手成功，进入传输阶段
			log.Printf("client %v negotiated\n", c.conn.RemoteAddr().String())
			c.status = NBD_Transmission
		case NBD_Transmission:
			if err := srv.transmission(c); err != nil {
				log.Printf("Transmission Err: %s\n", err.Error())
				return
			}
		}
	}
}

func (srv *NBDServer) negotiate(c *client) error {
	buf := make([]byte, 4096)

	init_magic := bytes.NewBuffer([]byte{})
	binary.Write(init_magic, binary.BigEndian, NBD_INIT_MAGIC)
	c.conn.Write(init_magic.Bytes())

	opt_magic := bytes.NewBuffer([]byte{})
	binary.Write(opt_magic, binary.BigEndian, NBD_INIT_MAGIC)
	c.conn.Write(opt_magic.Bytes())

	smallflag := NBD_FLAG_FIXED_NEWSTYLE | NBD_FLAG_NO_ZEROES
	flags := bytes.NewBuffer([]byte{})
	binary.Write(flags, binary.BigEndian, smallflag)
	c.conn.Write(flags.Bytes())

	n, err := c.conn.Read(buf[:4])
	if err != nil {
		return err
	}
	cflags_buf := bytes.NewBuffer(buf[:n])
	cflags := uint32(0)
	binary.Read(cflags_buf, binary.BigEndian, &cflags)
	if cflags&uint32(NBD_FLAG_NO_ZEROES) > 0 {

	}

	for {
		n, err := c.conn.Read(buf[:8])
		if err != nil {
			return err
		}
		magic := uint64(0)
		magic_buf := bytes.NewBuffer(buf[:n])
		binary.Read(magic_buf, binary.BigEndian, &magic)

		if magic != uint64(NBD_OPTS_MAGIC) {
			return errors.New("client not has NBD_OPTS_MAGIC")
		}

		opt := uint32(0)
		n, err = c.conn.Read(buf[:4])
		if err != nil {
			return err
		}
		opt_buf := bytes.NewBuffer(buf[:n])
		binary.Read(opt_buf, binary.BigEndian, &opt)

		switch opt {
		case uint32(NBD_OPT_EXPORT_NAME):
			return srv.handleExportName(c)
		case uint32(NBD_OPT_LIST):
			if err := srv.handleList(c, opt); err != nil {
				return err
			}
		case uint32(NBD_OPT_ABORT):
			/*
				客户端可以发送（并且服务器可以回复）一个 NBD_OPT_ABORT.
				这之后必须是客户端关闭 TLS（如果它正在运行），并且客户端断开连接。
				这被称为“启动软断开”；软断开只能由客户端发起。
			*/
			return errors.New("client call NBD_OPT_ABORT")
		case uint32(NBD_OPT_STARTTLS):
			// 尚不支持tls，只需消费无用的read
		case uint32(NBD_OPT_GO):
			fallthrough
		case uint32(NBD_OPT_INFO):
			if err := srv.handleInfo(c, opt); err != nil {
				return err
			} else if err == nil && opt == NBD_OPT_GO {
				return nil
			}
		default:
			return errors.New("unknown option")
		}
	}
}

func (srv *NBDServer) consume(c *client, size uint64) {
	buf := make([]byte, 1024)
	timeout := time.Second * 2
	readed := uint64(0)
	for size > 0 {
		if size > uint64(len(buf)) {
			readed = uint64(len(buf))
		} else {
			readed = uint64(size)
		}
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.conn.Read(buf)
		if os.IsTimeout(err) && n <= 0 {
			break
		}
		size -= readed
	}
	c.conn.SetReadDeadline(time.Time{})
}

// 查找client指定export name的server
func (srv *NBDServer) handleExportName(c *client) error {
	// 获取export name 长度
	buf := make([]byte, 4096)
	n, err := c.conn.Read(buf[:4])
	if err != nil {
		return err
	}
	namelen := uint32(0)
	namelen_buf := bytes.NewBuffer(buf[:n])
	binary.Read(namelen_buf, binary.BigEndian, &namelen)
	name := ""

	if namelen > 4096 {
		return errors.New("ExportName too long")
	}
	if namelen > 0 {
		n, err = c.conn.Read(buf[:namelen])
		if err != nil {
			return err
		}
		name = string(buf[:n])
	}
	if name == srv.exportName {
		srv.sendExportInfo(c)
		return nil
	}
	return errors.New("Negotiation failed/8a: Requested export not found, or is TLS-only and client did not negotiate TLS")
}

// 返回server 的 export name
func (srv *NBDServer) handleList(c *client, opt uint32) error {
	len_ := uint32(0)
	binary.Read(c.conn, binary.BigEndian, &len_)
	if len_ > 0 {
		srv.sendReply(c, opt, NBD_REP_ERR_INVALID, -1, []byte("NBD_OPT_LIST with nonzero data length is not a valid request"))
	}

	buf := bytes.NewBuffer([]byte{})
	binary.Write(buf, binary.BigEndian, uint32(len(srv.exportName)))
	buf.Write([]byte(srv.exportName))

	srv.sendReply(c, opt, NBD_REP_SERVER, -1, buf.Bytes())
	srv.sendReply(c, opt, NBD_REP_ACK, -1, nil)
	return nil
}

// Handle an NBD_OPT_INFO or NBD_OPT_GO request. 类似与handleExportName
func (srv *NBDServer) handleInfo(c *client, opt uint32) error {
	dataSize := uint32(0)
	binary.Read(c.conn, binary.BigEndian, &dataSize)
	nameLen := uint32(0)
	binary.Read(c.conn, binary.BigEndian, &nameLen)

	if nameLen > dataSize-6 {
		srv.sendReply(c, opt, NBD_REP_ERR_INVALID, -1, []byte("An OPT_INFO request cannot be smaller than the length of the name + 6"))
		srv.consume(c, uint64(dataSize)-uint64(nameLen))
	}
	if nameLen > 4096 {
		srv.sendReply(c, opt, NBD_REP_ERR_INVALID, -1, []byte("The name for this OPT_INFO request is too long"))
		srv.consume(c, uint64(nameLen))
	}
	name := ""
	buf := make([]byte, 4096)
	if nameLen > 0 {
		n, err := c.conn.Read(buf[:nameLen])
		if err != nil {
			return err
		}
		name = string(buf[:n])
	}

	if name != srv.exportName {
		return errors.New("cant find server which client need")
	}

	nRequests := uint16(0)
	binary.Read(c.conn, binary.BigEndian, &nRequests)

	sent_export := false
	req := uint16(128)
	for i := 0; i < int(nRequests); i++ {
		binary.Read(c.conn, binary.BigEndian, &req)
		switch req {
		case uint16(NBD_INFO_EXPORT):
			// req: 2bytes	export_size 8bytes flag: 2bytes
			srv.sendReply(c, opt, NBD_REP_INFO, 12, nil)
			binary.Write(c.conn, binary.BigEndian, req)
			srv.sendExportInfo(c)
			sent_export = true
		default:
			// ignore all other options for now.
		}
	}

	if !sent_export {
		req = uint16(NBD_INFO_EXPORT)
		srv.sendReply(c, opt, NBD_REP_INFO, 12, nil)
		binary.Write(c.conn, binary.BigEndian, req)
		srv.sendExportInfo(c)
	}

	srv.sendReply(c, opt, NBD_REP_ACK, 0, nil)

	return nil
}

func (srv *NBDServer) sendExportInfo(c *client) {
	binary.Write(c.conn, binary.BigEndian, srv.exportSize)
	binary.Write(c.conn, binary.BigEndian, NBD_FLAG_HAS_FLAGS|NBD_FLAG_SEND_WRITE_ZEROES)
}

func (srv *NBDServer) sendReply(c *client, opt uint32, replyType uint32, dataSize int64, data []byte) {
	buf := bytes.NewBuffer([]byte{})
	binary.Write(buf, binary.BigEndian, uint64(0x3e889045565a9))
	binary.Write(buf, binary.BigEndian, opt)
	binary.Write(buf, binary.BigEndian, replyType)
	if dataSize < 0 {
		binary.Write(buf, binary.BigEndian, uint32(len(data)))
	} else {
		binary.Write(buf, binary.BigEndian, uint32(dataSize))
	}

	if data != nil {
		buf.Write(data)
	}

	c.conn.Write(buf.Bytes())
}

// 传输阶段
func (srv *NBDServer) transmission(c *client) error {
	// buf := make([]byte, 4096)
	handle_err := func(c *client, rep *nbdReply, err uint32) {
		rep.err = err
		rep.Write(c.conn)
	}
	for {
		request := &nbdRequest{}
		rep := nbdReply{
			magic:  NBD_REPLY_MAGIC,
			err:    0,
			handle: request.handle,
		}
		err := request.Read(c.conn)
		if err != nil {
			return err
		}

		if request.magic != NBD_REQUEST_MAGIC {
			log.Println("request.magic!=NBD_REQUEST_MAGIC")
			handle_err(c, &rep, EINVAL)
			continue
		}
		cmd := request.reqType & NBD_CMD_MASK_COMMAND
		flags := request.reqType & (^NBD_CMD_MASK_COMMAND)
		// 若flags 未包含NBD_CMD_FLAG_FUA和NBD_CMD_FLAG_NO_HOLE标志则报错
		if (flags & ^(NBD_CMD_FLAG_FUA | NBD_CMD_FLAG_NO_HOLE)) > 0 {
			log.Printf("E: received invalid flag %v on command %v, ignoring", flags, cmd)
			return errors.Errorf("E: received invalid flag %v on command %v, ignoring", flags, cmd)
		}

		switch cmd {
		case NBD_CMD_READ:
			log.Printf("client want read block file from: %v len: %v\n", request.from, request.size)
			if err := srv.handleRead(c, request); err != nil {
				handle_err(c, &rep, EINVAL)
			}
		case NBD_CMD_WRITE:
			log.Printf("client want write block file from: %v len: %v\n", request.from, request.size)
			if err := srv.handleWrite(c, request); err != nil {
				handle_err(c, &rep, EINVAL)
			}
		case NBD_CMD_DISC:
			handle_err(c, &rep, EINVAL)
			log.Println("client disconnect")
			return errors.New("client disconnect")
		case NBD_CMD_FLUSH:
			srv.fd.Sync()
			log.Println("client call sync")
		default:
			handle_err(c, &rep, EINVAL)
		}

	}
}

func (srv *NBDServer) handleRead(c *client, req *nbdRequest) error {
	if req.from > srv.exportSize || req.from+uint64(req.size) > srv.exportSize {
		return errors.New("out of export size")
	}
	rep := &nbdReply{
		magic:  NBD_REPLY_MAGIC,
		err:    0,
		handle: req.handle,
	}
	rep.Write(c.conn)
	bufSize := 4096
	buf := make([]byte, bufSize)
	reqLen := int(req.size)
	srv.fd.Seek(int64(req.from), io.SeekStart)
	for {
		n, err := srv.fd.Read(buf)
		if err != nil {
			return err
		}
		if n >= int(reqLen) {
			c.conn.Write(buf[:reqLen])
			break
		}
		c.conn.Write(buf[:n])
		reqLen -= n
	}
	return nil
}

func (srv *NBDServer) handleWrite(c *client, req *nbdRequest) error {
	if req.from > srv.exportSize || req.from+uint64(req.size) > srv.exportSize {
		return errors.New("out of export size")
	}
	srv.fd.Seek(int64(req.from), io.SeekStart)
	bufSize := 4096
	buf := make([]byte, bufSize)
	writed := 0
	needRead := bufSize
	// 需要防止读取过多，导致误读取后续请求
	for {
		leftRead := req.size - uint32(writed)
		if bufSize >= int(leftRead) {
			needRead = int(leftRead)
		} else {
			needRead = bufSize
		}

		n, err := c.conn.Read(buf[:needRead])
		if err != nil {
			return err
		}
		srv.fd.Write(buf[:n])
		writed += n
		log.Printf("client write block from %v len %v had writed %v\n", req.from, req.size, writed)
		if writed >= int(req.size) {
			break
		}
	}
	rep := nbdReply{
		magic:  NBD_REPLY_MAGIC,
		err:    0,
		handle: req.handle,
	}
	rep.Write(c.conn)
	return nil
}

// 处理退出
func (srv *NBDServer) handleQuit(sigs chan os.Signal) {
	s := <-sigs
	if s == os.Interrupt || s == os.Kill {
		srv.close()
	}
}

// 设置退出标志，并关闭所有client
func (srv *NBDServer) close() {
	log.Println("nbdserver exit")
	atomic.StoreUint32(&srv.quit, 1)
	srv.listener.Close()
	for i := 0; i < len(srv.clients); i++ {
		srv.clients[i].conn.Close()
	}
}
