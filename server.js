/*eslint no-console: 0*/
const express = require('express');
const app = express();
const server = require('http').createServer(app);
const io = require('socket.io')(server);


////////////////////////////////////////////////////////////////////////////////
// ExpressJS

app.use(express.static('public'));
app.use('/ping', (req, res) => {
    res.send({"message": "pong"});
});

////////////////////////////////////////////////////////////////////////////////
// Socket.IO

const clients = {};
let senderId;

io.on('connection', (socket) => {
    console.log('[%s] new connection', socket.id);

    clients[socket.id] = {
        since: Date.now(),
        socket: socket,
        isSender: false, // don't know yet
    };

    socket.on('disconnect', function () {
        console.log('[%s] disconnect', socket.id);
        const c = clients[socket.id];
        if (c.isSender) {
            senderId = null;
        }
        delete clients[socket.id];
    });

    // message for server
    socket.on('syn', function (ev) {
        console.log('[%s] syn received:', socket.id, ev);
        const c = clients[socket.id];
        if (!c) {
            socket.emit('syn-ack', {
                success: false,
                reason: 'not registered',
                msgId: ev.msgId
            });
            return;
        }
        c.isSender = ev.isSender;
        if (c.isSender) {
            if (senderId) {
                if (senderId === socket.id) {
                    console.warn("already a master");
                } else {
                    console.warn("invalidate the current senderId");
                }
            }
            senderId = socket.id;
        } else {
            console.log("syn received from a receiver");
        }
        console.log("sending syn-ack: senderId:%s", senderId);
        socket.emit('syn-ack', {
            success: true,
            senderId: senderId,
            msgId: ev.msgId
        });
    });

    // message for peer
    socket.on('sig', function (ev) {
        console.log('[%s] sig received:', socket.id, ev);
        const me = clients[socket.id];
        if (!me) {
            socket.emit('sig-ack', {
                success: false,
                reason: "not sync'd",
                msgId: ev.msgId
            });
            return;
        }
        if (!ev.to) {
            socket.emit('sig-ack', {
                success: false,
                reason: '"to" field is missing',
                msgId: ev.msgId
            });
            return;
        }

        const peer = clients[ev.to];
        if (!peer) {
            socket.emit('sig-ack', {
                success: false,
                reason: 'peer not found',
                msgId: ev.msgId
            });
            return;
        }

        // Add `from` field, then forward to the destination.
        ev.from = socket.id;
        peer.socket.emit('sig', ev);
    });
});

server.listen(8080);

