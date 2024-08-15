use std::{
    ffi::{c_char, c_void, CStr},
    io::{self, Cursor},
    net::{SocketAddr, UdpSocket},
    ptr::null_mut,
    slice,
};

use anyhow::{Context, Error};

pub struct udpsocket_error(Error);

#[no_mangle]
pub unsafe extern "C" fn udpsocket_error_drop(err: *mut udpsocket_error) {
    assert!(!err.is_null());

    let err = &mut *err;
    drop(Box::from_raw(err));
}

#[no_mangle]
pub unsafe extern "C" fn udpsocket_error_print(
    err: *const udpsocket_error,
    buf: *mut u8,
    buf_len: usize,
) -> usize {
    assert!(!err.is_null());
    assert!(!buf.is_null());

    let err = &*err;
    let buf = slice::from_raw_parts_mut(buf, buf_len);
    let mut cursor = Cursor::new(buf);

    use io::Write;
    // QUOTE:
    // > To print causes as well using anyhow’s default formatting of causes, use the alternate
    // > selector “{:#}”.
    // - https://docs.rs/anyhow/latest/anyhow/struct.Error.html#display-representations
    let _ = write!(cursor, "{:#}", &err.0);
    cursor.position() as usize
}

unsafe fn parse_addr(addr: *const c_char) -> Result<SocketAddr, Error> {
    assert!(!addr.is_null());

    let addr = CStr::from_ptr(addr);
    let addr = addr.to_str().context("to_str")?;
    let addr = addr.parse::<SocketAddr>().context("parse")?;

    Ok(addr)
}

#[no_mangle]
pub unsafe extern "C" fn udpsocket_bind(
    addr: *const c_char,
    socket_out: *mut *mut c_void,
) -> *mut udpsocket_error {
    assert!(!addr.is_null());
    assert!(!socket_out.is_null());

    let addr = match parse_addr(addr).context("parse_addr") {
        Ok(addr) => addr,
        Err(err) => return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error,
    };

    let socket = match UdpSocket::bind(&addr).context("bind") {
        Ok(socket) => socket,
        Err(err) => return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error,
    };

    *socket_out = Box::into_raw(Box::new(socket)) as *mut c_void;
    null_mut()
}

#[no_mangle]
pub unsafe extern "C" fn udpsocket_drop(socket: *mut c_void) {
    assert!(!socket.is_null());

    let socket = &mut *(socket as *mut UdpSocket);
    drop(Box::from_raw(socket));
}

#[no_mangle]
pub unsafe extern "C" fn udpsocket_set_nonblocking(
    socket: *mut c_void,
    nonblocking: bool,
) -> *mut udpsocket_error {
    assert!(!socket.is_null());

    let socket = &mut *(socket as *mut UdpSocket);
    if let Err(err) = socket
        .set_nonblocking(nonblocking)
        .context("set_nonblocking")
    {
        return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error;
    }

    null_mut()
}

#[no_mangle]
pub unsafe extern "C" fn udpsocket_connect(
    socket: *mut c_void,
    addr: *const c_char,
) -> *mut udpsocket_error {
    assert!(!socket.is_null());
    assert!(!addr.is_null());

    let addr = match parse_addr(addr).context("parse_addr") {
        Ok(addr) => addr,
        Err(err) => return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error,
    };

    let socket = &mut *(socket as *mut UdpSocket);
    if let Err(err) = socket.connect(&addr).context("connect") {
        return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error;
    }

    null_mut()
}

/// The send method of a connected UdpSocket does not guarantee delivery of packets, as UDP is
/// inherently an unreliable protocol. If the server is not up or reachable, the send method may
/// not fail immediately. Instead, it can return successfully, indicating that the data has been
/// sent to the kernel's socket buffer, even if it is not actually delivered to the server.
#[no_mangle]
pub unsafe extern "C" fn udpsocket_send(
    socket: *mut c_void,
    buf: *const u8,
    buf_len: usize,
    n_out: *mut usize,
) -> *mut udpsocket_error {
    assert!(!socket.is_null());
    assert!(!buf.is_null());
    assert!(!n_out.is_null());

    let socket = &mut *(socket as *mut UdpSocket);
    let buf = slice::from_raw_parts(buf, buf_len);
    let n = match socket.send(buf).context("send") {
        Ok(n) => n,
        Err(err) => return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error,
    };

    *n_out = n;
    null_mut()
}

#[no_mangle]
pub unsafe extern "C" fn udpsocket_recv(
    socket: *mut c_void,
    buf: *mut u8,
    buf_len: usize,
    n_out: *mut usize,
) -> *mut udpsocket_error {
    assert!(!socket.is_null());
    assert!(!buf.is_null());
    assert!(!n_out.is_null());

    let socket = &mut *(socket as *mut UdpSocket);
    let buf = slice::from_raw_parts_mut(buf, buf_len);
    let n = match socket.recv(buf).context("recv") {
        Ok(n) => n,
        Err(err) => match err.downcast_ref::<io::Error>() {
            Some(err) if err.kind() == io::ErrorKind::WouldBlock => 0,
            _ => return Box::into_raw(Box::new(udpsocket_error(err))) as *mut udpsocket_error,
        },
    };

    *n_out = n;
    null_mut()
}

#[cfg(test)]
mod tests {
    use std::alloc::{alloc, Layout};

    use super::*;

    unsafe fn check_err(err: *mut udpsocket_error) -> bool {
        if err.is_null() {
            return false;
        }
        const BUF_LEN: usize = 1024;
        let mut buf = [0u8; BUF_LEN];
        let n = udpsocket_error_print(err, &mut buf as *mut u8, BUF_LEN);
        udpsocket_error_drop(err);
        eprintln!("err: {}", std::str::from_utf8_unchecked(&buf[..n]));
        true
    }

    #[test]
    fn it_works() {
        unsafe {
            let udpsocket = alloc(Layout::new::<UdpSocket>()) as *mut *mut c_void;
            let err = udpsocket_bind(c"127.0.0.1:34254".as_ptr(), udpsocket as *mut *mut c_void);
            assert!(!check_err(err));

            let err = udpsocket_connect(*udpsocket, c"127.0.0.1:5000".as_ptr());
            assert!(!check_err(err));

            const BUF_LEN: usize = 1024;
            let mut buf = [0u8; BUF_LEN];
            buf[0] = 1;

            let mut n_out: usize = 0;
            let err = udpsocket_send(*udpsocket, &buf as *const u8, 1, &mut n_out as *mut usize);
            assert!(!check_err(err));

            let mut n_out: usize = 0;
            let err = udpsocket_recv(*udpsocket, &mut buf as *mut u8, 1, &mut n_out as *mut usize);
            assert!(check_err(err));
        }
    }
}
