'use strict';
/*eslint no-console: 0*/

const audioElm = document.getElementById('audioElm');
const sdpoutElm = document.getElementById("sdpoutElm");
const sdpinElm = document.getElementById("sdpinElm");


let cc;
let candidates = [];

function call() {
    console.log('Starting call');
    const servers = null;
    cc = new RTCPeerConnection(servers);
    console.log('Created local peer connection object cc');
    cc.onicecandidate = (e) => onIceCandidate(e);
    cc.oniceconnectionstatechange = (e) => {
        console.log(`client: ICE state: ${cc.iceConnectionState}`);
    };
    cc.ontrack = onTrack;
    cc.onsignalingstatechange = (event) => {
        console.log('client: signaling state: %s', cc.signalingState);
    };

    console.log('client: createOffer start');
    cc.createOffer(onCreateOfferSuccess, (err) => {
        console.log('client: createOffer failed:', err);
    }, {
        offerToReceiveAudio: true,
        offerToReceiveVideo: false
    });
}

function onSDPReceived(desc) {
    console.log('client: setRemoteDescription start');
    cc.setRemoteDescription(desc, () => {
        console.log('client: setRemoteDescription complete');
    }, (err) => {
        console.log('client: setRemoteDescription failed:', err);
    });

    if (candidates.length > 0) {
        sdpoutElm.value = btoa(JSON.stringify(candidates));
        candidates = [];
    }
}

function onCreateOfferSuccess(desc) {
    const sDesc = JSON.stringify(desc);
    console.log(`Offer from client: ${sDesc}`);
    console.log('client: setLocalDescription start');
    cc.setLocalDescription(desc, () => {
        console.log('client: setLocalDescription complete');
    }, (err) => {
        console.log('client: setLocalDescription failed:', err);
    });

    sdpoutElm.value = btoa(sDesc);
}

function onTrack(event) {
    if (audioElm.srcObject !== event.streams[0]) {
        audioElm.srcObject = event.streams[0];
        console.log('client: received remote stream', event);
    }
}

function onIceCandidate(event) {
    console.log(`client: ICE candidate: ${event.candidate}`);
    candidates.push(event.candidate);
}

function onSDPEntered() {
    const sdp = atob(sdpinElm.value);
    const desc = JSON.parse(sdp);
    sdpinElm.value = "";
    sdpoutElm.value = "";
    console.log("Entered SDP:", desc);
    if (Array.isArray(desc)) {
        desc.forEach((cand) => {
            cc.addIceCandidate(cand).then(() => {
                console.log('client: addIceCandidate success');
            }, (err) => {
                console.log('client: addIceCandidate failed:', err);
            });
        });

        if (candidates.length > 0) {
            sdpoutElm.value = btoa(JSON.stringify(candidates));
            candidates = [];
        }
    } else {
        if (desc.hasOwnProperty("type")) {
            if (desc.type === "offer") {
                console.log("Unexpected SDP type: %s", desc.type);
            } else if (desc.type === "answer" || desc.type === "pranswer") {
                onSDPReceived(desc);
            } else {
                console.log("Unsupported SDP type: %s", desc.type);
            }
        } else {
            console.log("Unknown SDP entered: %s", desc);
        }
    }
}

call();
