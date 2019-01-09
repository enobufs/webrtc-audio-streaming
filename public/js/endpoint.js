'use strict';
/*eslint no-console: 0*/

const statusElm = document.getElementById("statusElm");
const connstElm = document.getElementById("connstElm");
const audioElm = document.getElementById('audioElm');
const buActionElm = document.getElementById("buActionElm");

let audioSource; // master only
let trickle = false;

function setUpStream() {
    return new Promise((resolve, reject) => {
        // Audio tag capture must be set up after audio tracks are enumerated.
        audioElm.oncanplay = () => {
            _maybeCreateStream(resolve, reject);
        };
        if (audioElm.readyState >= 3) {
            _maybeCreateStream(resolve, reject);
        } else {
            resolve(); // wait oncanplay to be called
        }
    });
}

function _maybeCreateStream(resolve, reject) {
    if (audioSource) {
        resolve();
    }
    if (audioElm.captureStream) {
        audioSource = audioElm.captureStream();
        console.log('Captured stream from audioElm with captureStream', audioSource);
        resolve();
        return;
    }
    if (audioElm.mozCaptureStream) {
        audioSource = audioElm.mozCaptureStream();
        console.log('Captured stream from audioElm with mozCaptureStream()', audioSource);
        resolve();
        return;
    }

    reject(new Error('captureStream() not supported'));
}

class Endpoint {
    constructor() {
        this._candidates = [];
        this._peerId;
        this._state = "";
    }

    get state() {
        return this._state;
    }

    start() {
        // Common setup
        if (isSender) {
            // Sender won't initiate connection.
            return;
        }

        this._createNewConnection();
        this._pc.ontrack = this._onTrack;
        this._pc.onsignalingstatechange = (event) => {
            console.log('signaling state: %s', this._pc.signalingState);
        };

        console.log('createOffer start');
        this._pc.createOffer(this._onCreateOfferSuccess.bind(this), (err) => {
            console.log('createOffer failed:', err);
        }, {
            offerToReceiveAudio: true,
            offerToReceiveVideo: false
        });
    }

    close() {
        if (!this._pc) {
            return;
        }
        this._pc.close();
    }

    _createNewConnection() {
        console.log('Starting creating peer connection');
        const servers = null;
        this._pc = new RTCPeerConnection(servers);
        console.log('Created local peer connection object this._pc');
        this._pc.onicecandidate = (e) => this._onIceCandidate(e);
        this._pc.oniceconnectionstatechange = (e) => {
            const from = this._state;
            this._state = this._pc.iceConnectionState;
            onConnectionStateChange({
                from: from,
                to: this._state
            });
        };
        this._state = this._pc.iceConnectionState;
    }

    _onSDPReceived(desc) {
        console.log('received %s', desc.type);
        console.dir(desc);

        if (isSender) {
            this._createNewConnection();
            audioSource.getTracks().forEach((track) => this._pc.addTrack(track, audioSource));
            console.log('added local stream to pc');

            console.log("remote description:", desc);
            this._pc.setRemoteDescription(desc, () => {
                console.log(`${myRole}: setRemoteDescription complete`);
            }, (err) => {
                console.log(`${myRole}: setRemoteDescription failed:`, err);
            });

            // Create answer
            console.log('createAnswer start');
            this._pc.createAnswer(this._onCreateAnswerSuccess.bind(this), (err) => {
                console.log('createAnswer failed:', err);
            });
        } else {
            this._pc.setRemoteDescription(desc, () => {
                console.log(`${myRole}: setRemoteDescription complete`);
            }, (err) => {
                console.log(`${myRole}: setRemoteDescription failed:`, err);
            });
        }
    }

    _onCreateOfferSuccess(desc) {
        const sDesc = JSON.stringify(desc);
        console.log(`created offer: ${sDesc}`);
        console.log('offerer: setLocalDescription start');
        this._pc.setLocalDescription(desc, () => {
            console.log('offerer: setLocalDescription complete');
        }, (err) => {
            console.log('offerer: setLocalDescription failed:', err);
        });

        if (trickle) {
            // send offer
            console.log("sending offer to %s", senderId);
            sig.send('sig', {
                type: 'description',
                to: senderId,
                body: desc
            }).then(() => {
                console.log("offer acked!");
            });
        } else {
            console.log("defer sending offer to %s", senderId);
        }
    }

    _onCreateAnswerSuccess(desc) {
        const sDesc = JSON.stringify(desc);
        console.log(`created answer: ${sDesc}`);
        console.log('setLocalDescription start');
        this._pc.setLocalDescription(desc, () => {
            console.log('setLocalDescription complete');
        }, (err) => {
            console.log('setLocalDescription failed:', err);
        });

        if (trickle) {
            // send answer
            console.log("sending answer to %s", this._peerId);
            sig.send('sig', {
                type: 'description',
                to: this._peerId,
                body: desc
            }).then(() => {
                console.log("offer acked!");
            });
        } else {
            console.log("defer sending answer to %s", this._peerId);
        }
    }

    _onTrack(event) {
        if (audioElm.srcObject !== event.streams[0]) {
            audioElm.srcObject = event.streams[0];
            console.log('offerer: received remote stream', event);
        }
    }

    _onIceCandidate(event) {
        if (trickle) {
            if (event.candidate) {
                console.log('sending ICE candidate:', event.candidate);

                sig.send('sig', {
                    type: 'candidate',
                    to: isSender ? this._peerId : senderId,
                    body: event.candidate
                });
            } else {
                console.log('end of ICE candidate');
            }
        } else {
            if (event.candidate) {
                console.log('collecting ICE candidate:', event.candidate);

                this._candidates.push(event.candidate);
            } else {
                console.log('end of ICE candidate');

                const desc = this._pc.localDescription;

                // send offer or answer now.
                const to = isSender ? this._peerId : senderId;
                console.log("sending offer to %s", to);
                sig.send('sig', {
                    type: 'description',
                    to: to,
                    body: desc
                }).then(() => {
                    console.log("offer acked!");
                });
            }
        }
    }

    _sendCandidates() {
        if (this._candidates.length > 0) {
            console.log("sending candidates...");
            const promises = [];
            while(this._candidates.length > 0) {
                let cand = this._candidates.shift();
                promises.push(sig.send('sig', {
                    type: 'candidate',
                    to: this._peerId,
                    body: cand
                }).then(() => {
                    console.log("candidate acked!");
                }, (err) => {
                    console.error(err);
                    console.dir(cand);
                }));
            }

            return Promise.all(promises);
        }

        console.log("no candidates to send...");
    }

    onSDPEntered(desc, from) {
        if (desc.hasOwnProperty("type")) {
            this._peerId = from;
            if (desc.type === "offer") {
                console.assert(this._peerId !== senderId, {
                    peerId: this._peerId,
                    senderId: senderId
                });
                this._onSDPReceived(desc);
            } else if (desc.type === "answer" || desc.type === "pranswer") {
                console.assert(this._peerId === senderId, {
                    peerId: this._peerId,
                    senderId: senderId
                });
                this._onSDPReceived(desc);
            } else {
                console.log("Unsupported SDP type: %s", desc.type);
            }
        } else {
            console.dir(desc);
        }
    }

    onCandidateReceived(candidate) {
        this._pc.addIceCandidate(candidate).then(() => {
            console.log('addIceCandidate success');
        }, (err) => {
            console.log('addIceCandidate failed:', err);
        });
    }
}

function onStart() {
    setUpStream().then(() => {
        console.log("source stream ready");
        if (sig.state === SigState.Offline) {
            //audioElm.play();
            sig.connect();
        } else if (sig.state === SigState.Online) {
            sig.disconnect();
        } else {
            // Nothing to do
        }
    });
}

function onSigOffline() {
    console.log('signaling disconnected by remote');
    statusElm.innerText = "Go Online";
}

function onSigStateChange(ev) {
    switch (ev.to) {
        case SigState.Online:
            statusElm.innerText = "online";
            break;
        case SigState.Connecting:
            statusElm.innerText = "connecting...";
            break;
        case SigState.Offline:
            statusElm.innerText = "offline";
            break;
    }
}

if (isSender) {
    buActionElm.disabled = true;
}

function onConnectionStateChange(ev) {
    console.log(`Connction state changed from %s to %s`, ev.from, ev.to);
    connstElm.innerText = ev.to;
    switch (ev.to) {
        case "new":
            break;
        case "checking":
            buActionElm.disabled = true;
            buActionElm.innerHTML = "Disconnect";
            break;
        case "connected":
            buActionElm.disabled = false;
            break;
        case "completed":
            buActionElm.disabled = false;
            break;
        case "failed":
            buActionElm.disabled = false;
            break;
        case "disconnected":
        case "closed":
            buActionElm.innerHTML = "Connect";
            if (isSender) {
                buActionElm.disabled = true;
            } else {
                buActionElm.disabled = false;
            }
            break;
    }
}

function onAction() {
    switch (ep.state) {
        case "":
        case "disconnected":
        case "closed":
            if (!isSender) {
                ep.start();
            }
            break;
        case "connected":
        case "completed":
        case "failed":
            ep.close();
            break;
        default:
            break;
    }
}

const sig = new Signaling();
const ep = new Endpoint();

