/*eslint no-console: 0*/
'use strict';


// Event ID
// Client-Server (no "to" field)
//  - "syn" C->S, "ack" by S
//     o tells seerver if the client is a sender.
//     o sender ID is returned in the ack
// Client-(server)-Client
//  - "sig", "ack" by peer
//     types:
//        o "description" => offer, answer
//        o "candidate"
//        o "bye"

let senderId;

const SigState = {
    Offline: 0,
    Connecting: 1,
    Online: 2
};

class Signaling {
    constructor(onOnline, onOffline) {
        this._onOnline = onOnline;
        this._onOffline = onOffline;
        this._state = SigState.Offline;
        this._socket = null;
        this._lastMsgId = 0;
        this._msgs = {};
    }

    get state() {
        return this._state;
    }

    connect() {
        const onAck = (ev) => {
            console.log('ack received');
            console.dir(ev);
            const ack = this._msgs[ev.msgId];
            if (!ack) {
                console.warn("msg to resolve on ack not found");
                return;
            }
            const promise = ack.promise;
            if (!Array.isArray(promise) || promise.length != 2) {
                console.warn("promise must be an array with length 2");
                return;
            }
            if (!ev.success) {
                const err = new Error(ev.reason);
                err.sent = ack.sent;
                promise[1](err);
                return;
            }
            promise[0](ev);
        };

        if (this._socket) {
            return Promise.reject(new Error("already has socket"));
        }
        this._setState(SigState.Connecting);

        // Connect to the signaling server
        console.log('creating socket...');
        this._socket = io.connect({
            transports: ['websocket']
        });

        this._socket.on('syn-ack', onAck);
        this._socket.on('sig-ack', onAck);

        this._socket.on('pub', (ev) => {
            void(ev);
        });

        this._socket.on('sig', (ev) => {
            console.log("sig event received: type=%s", ev.type);
            if (!ep) {
                console.log("ep not available");
                this._socket.emit('ack', {
                    success: false,
                    reason: "endpoint not initialized",
                    to: ev.from,
                    msgId: ev.msgid
                });
                return;
            }

            console.log("emiting ack");
            this._socket.emit('sig-ack', {
                success: true,
                to: ev.from,
                msgId: ev.msgid
            });

            if (ev.type === "description") {
                ep.onSDPEntered(ev.body, ev.from);
                return;
            }

            if (ev.type === "candidate") {
                ep.onCandidateReceived(ev.body);
                return;
            }
        });

        console.log("sending syn..");
        return this.send('syn', {
            name: "anonymous",
            isSender: isSender
        }).then((res) => {
            senderId = res.senderId;
            this._setState(SigState.Online);
        });
    }

    disconnect() {
        // Connect to the signaling server
        console.log('closing socket...');
        this._setState(SigState.Offline);
        this._socket.close();
    }

    send(evId, ev) {
        return new Promise((resolve, reject) => {
            ev.msgId = ++this._lastMsgId;
            this._msgs[ev.msgId] = {
                sent: JSON.stringify(ev),
                promise: [resolve, reject]
            };
            this._socket.emit(evId, ev);
        });
    }

    _setState(to) {
        const from = this._state;
        this._state = to;
        onSigStateChange({ from: from, to: to });
    }
}

