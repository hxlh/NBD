package nbdserver

// magic
const (
	NBD_INIT_MAGIC = int64(0x4e42444d41474943) /* ASCII "NBDMAGIC" */
	NBD_OPTS_MAGIC = int64(0x49484156454F5054) /* ASCII "IHAVEOPT" */
)

/* Options that the client can select to the server */
const (
	NBD_OPT_EXPORT_NAME = uint32(1) /**< Client wants to select a named export (is followed by name of export) */
	NBD_OPT_ABORT       = uint32(2) /**< Client wishes to abort negotiation */
	NBD_OPT_LIST        = uint32(3) /**< Client request list of supported exports (not followed by data) */
	NBD_OPT_STARTTLS    = uint32(5) /**< Client wishes to initiate TLS */
	NBD_OPT_INFO        = uint32(6) /**< Client wants information about the given export */
	NBD_OPT_GO          = uint32(7) /**< Client wants to select the given and move to the transmission phase */
)

/* Replies the server can send during negotiation */
const (
	NBD_REP_ACK                 = uint32(1)                      /**< ACK a request. Data: option number to be acked */
	NBD_REP_SERVER              = uint32(2)                      /**< Reply to NBD_OPT_LIST (one of these per server; must be followed by NBD_REP_ACK to signal the end of the list */
	NBD_REP_INFO                = uint32(3)                      /**< Reply to NBD_OPT_INFO */
	NBD_REP_FLAG_ERROR          = uint32(1 << 31)                /** If the high bit is set, the reply is an error */
	NBD_REP_ERR_UNSUP           = uint32(1 | NBD_REP_FLAG_ERROR) /**< Client requested an option not understood by this version of the server */
	NBD_REP_ERR_POLICY          = uint32(2 | NBD_REP_FLAG_ERROR) /**< Client requested an option not allowed by server configuration. (e.g., the option was disabled) */
	NBD_REP_ERR_INVALID         = uint32(3 | NBD_REP_FLAG_ERROR) /**< Client issued an invalid request */
	NBD_REP_ERR_PLATFORM        = uint32(4 | NBD_REP_FLAG_ERROR) /**< Option not supported on this platform */
	NBD_REP_ERR_TLS_REQD        = uint32(5 | NBD_REP_FLAG_ERROR) /**< TLS required */
	NBD_REP_ERR_UNKNOWN         = uint32(6 | NBD_REP_FLAG_ERROR) /**< NBD_OPT_INFO or ..._GO requested on unknown export */
	NBD_REP_ERR_BLOCK_SIZE_REQD = uint32(8 | NBD_REP_FLAG_ERROR) /**< Server is not willing to serve the export without the block size being negotiated */
)

/* Global flags */
const (
	NBD_FLAG_FIXED_NEWSTYLE = uint16(1) << 0
	NBD_FLAG_NO_ZEROES      = uint16(1) << 1
)

/* Info types */
const (
	NBD_INFO_EXPORT      = int(0)
	NBD_INFO_NAME        = int(1)
	NBD_INFO_DESCRIPTION = int(2)
	NBD_INFO_BLOCK_SIZE  = int(3)
)

/* values for flags field */
const (
	NBD_FLAG_HAS_FLAGS         = uint16(1 << 0) /* Flags are there */
	NBD_FLAG_READ_ONLY         = uint16(1 << 1) /* Device is read-only */
	NBD_FLAG_SEND_FLUSH        = uint16(1 << 2) /* Send FLUSH */
	NBD_FLAG_SEND_FUA          = uint16(1 << 3) /* Send FUA (Force Unit Access) */
	NBD_FLAG_ROTATIONAL        = uint16(1 << 4) /* Use elevator algorithm - rotational media */
	NBD_FLAG_SEND_TRIM         = uint16(1 << 5) /* Send TRIM (discard) */
	NBD_FLAG_SEND_WRITE_ZEROES = uint16(1 << 6) /* Send NBD_CMD_WRITE_ZEROES */
	NBD_FLAG_CAN_MULTI_CONN    = uint16(1 << 8) /* multiple connections are okay */
)

/* These are sent over the network in the request/reply magic fields */

const (
	NBD_REQUEST_MAGIC          = uint32(0x25609513)
	NBD_REPLY_MAGIC            = uint32(0x67446698)
	NBD_STRUCTURED_REPLY_MAGIC = uint32(0x668e33ef)
)

const (
	NBD_CMD_MASK_COMMAND = uint32(0x0000ffff)
	NBD_CMD_SHIFT        = uint32((16))
	NBD_CMD_FLAG_FUA     = uint32(((1 << 0) << NBD_CMD_SHIFT))
	NBD_CMD_FLAG_NO_HOLE = uint32(((1 << 1) << NBD_CMD_SHIFT))
)

const (
	NBD_CMD_READ         = 0
	NBD_CMD_WRITE        = 1
	NBD_CMD_DISC         = 2
	NBD_CMD_FLUSH        = 3
	NBD_CMD_TRIM         = 4
	NBD_CMD_CACHE        = 5
	NBD_CMD_WRITE_ZEROES = 6
	NBD_CMD_BLOCK_STATUS = 7
	NBD_CMD_RESIZE       = 8
)

const (
	EPERM   = uint32(1)  /* Operation not permitted */
	ENOENT  = uint32(2)  /* No such file or directory */
	ESRCH   = uint32(3)  /* No such process */
	EINTR   = uint32(4)  /* Interrupted system call */
	EIO     = uint32(5)  /* I/O error */
	ENXIO   = uint32(6)  /* No such device or address */
	E2BIG   = uint32(7)  /* Argument list too long */
	ENOEXEC = uint32(8)  /* Exec format error */
	EBADF   = uint32(9)  /* Bad file number */
	ECHILD  = uint32(10) /* No child processes */
	EAGAIN  = uint32(11) /* Try again */
	ENOMEM  = uint32(12) /* Out of memory */
	EACCES  = uint32(13) /* Permission denied */
	EFAULT  = uint32(14) /* Bad address */
	ENOTBLK = uint32(15) /* Block device required */
	EBUSY   = uint32(16) /* Device or resource busy */
	EEXIST  = uint32(17) /* File exists */
	EXDEV   = uint32(18) /* Cross-device link */
	ENODEV  = uint32(19) /* No such device */
	ENOTDIR = uint32(20) /* Not a directory */
	EISDIR  = uint32(21) /* Is a directory */
	EINVAL  = uint32(22) /* Invalid argument */
	ENFILE  = uint32(23) /* File table overflow */
	EMFILE  = uint32(24) /* Too many open files */
	ENOTTY  = uint32(25) /* Not a typewriter */
	ETXTBSY = uint32(26) /* Text file busy */
	EFBIG   = uint32(27) /* File too large */
	ENOSPC  = uint32(28) /* No space left on device */
	ESPIPE  = uint32(29) /* Illegal seek */
	EROFS   = uint32(30) /* Read-only file system */
	EMLINK  = uint32(31) /* Too many links */
	EPIPE   = uint32(32) /* Broken pipe */
	EDOM    = uint32(33) /* Math argument out of domain of func */
	ERANGE  = uint32(34) /* Math result not representable */
)
