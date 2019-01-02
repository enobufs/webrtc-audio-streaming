'use strict';
/*eslint no-console: 0*/

const audioElm = document.getElementById('audioElm');
const sdpoutElm = document.getElementById("sdpoutElm");
const sdpinElm = document.getElementById("sdpinElm");

let stream;
let sc;
let candidates = [];

function maybeCreateStream() {
    if (stream) {
        return;
    }
    if (audioElm.captureStream) {
        stream = audioElm.captureStream();
        console.log('Captured stream from audioElm with captureStream',
            stream);
        call();
    } else if (audioElm.mozCaptureStream) {
        stream = audioElm.mozCaptureStream();
        console.log('Captured stream from audioElm with mozCaptureStream()',
            stream);
        call();
    } else {
        console.log('captureStream() not supported');
    }
}

// Audio tag capture must be set up after audio tracks are enumerated.
audioElm.oncanplay = maybeCreateStream;
if (audioElm.readyState >= 3) {
    maybeCreateStream();
}

function call() {
    console.log('Starting call');
    const audioTracks = stream.getAudioTracks();
    if (audioTracks.length > 0) {
        console.log(`Using audio device: ${audioTracks[0].label}`);
    }
    const servers = null;
    sc = new RTCPeerConnection(servers);
    console.log('Created remote peer connection object sc');
    sc.onicecandidate = (e) => onIceCandidate(e);
    sc.oniceconnectionstatechange = (e) => onIceStateChange(sc, e);

    stream.getTracks().forEach((track) => sc.addTrack(track, stream));
    console.log('Added local stream to sc');
}

function onSDPReceived(desc) {
    console.log('sc setRemoteDescription start');
    sc.setRemoteDescription(desc, () => {
        console.log('server: setRemoteDescription complete');
    }, (err) => {
        console.log('server: setRemoteDescription failed:', err);
    });
    console.log('sc createAnswer start');
    sc.createAnswer(onCreateAnswerSuccess, (err) => {
        console.log('server: createAnswer failed:', err);
    });
}

function onCreateAnswerSuccess(desc) {
    const sDesc = JSON.stringify(desc);
    console.log(`Answer from sc: ${sDesc}`);
    console.log('sc setLocalDescription start');
    sc.setLocalDescription(desc, () => {
        console.log('server: setLocalDescription complete');
    }, (err) => {
        console.log('server: setLocalDescription failed:', err);
    });

    sdpoutElm.value = btoa(sDesc);
}

function onIceCandidate(event) {
    console.log(`server: ICE candidate: ${event.candidate}`);
    candidates.push(event.candidate);
}

function onIceStateChange(pc, event) {
    if (pc) {
        console.log(`server: ICE state: ${pc.iceConnectionState}`);
        console.log('ICE state change event: ', event);
    }
}

function onSDPEntered() {
    // Start playing audio on the first click.
    // (Chrome won't auto-play without a human interaction)
    audioElm.play();

    const sdp = atob(sdpinElm.value);
    const desc = JSON.parse(sdp);
    sdpinElm.value = "";
    sdpoutElm.value = "";
    console.log("Entered SDP:", desc);
    if (Array.isArray(desc)) {
        desc.forEach((cand) => {
            sc.addIceCandidate(cand).then(() => {
                console.log('server: addIceCandidate success');
            }, (err) => {
                console.log('server: addIceCandidate failed:', err);
            });
        });

        if (candidates.length > 0) {
            sdpoutElm.value = btoa(JSON.stringify(candidates));
            candidates = [];
        }
    } else {
        if (desc.hasOwnProperty("type")) {
            if (desc.type === "offer") {
                onSDPReceived(desc);
            } else if (desc.type === "answer" || desc.type === "pranswer") {
                console.log("Unexpected SDP type: %s", desc.type);
            } else {
                console.log("Unsupported SDP type: %s", desc.type);
            }
        } else {
            console.log("Unknown SDP entered: %s", desc);
        }
    }
}
