const joinBtn = document.getElementById("joinCustom");
const keyInput = document.getElementById("customKey");
const msgInput = document.getElementById("messageInput");
const sendBtn = document.getElementById("sendBtn");

let socket = null;

function setConnected(connected) {
  joinBtn.disabled = connected;
  keyInput.disabled = connected;
  msgInput.disabled = !connected;
  sendBtn.disabled = !connected;
}

joinBtn.addEventListener("click", () => {
  const room = keyInput.value.trim();
  if (!room) {
    alert("Please enter a room key.");
    return;
  }
  socket = new WebSocket(`ws://localhost:8080/ws/${room}`);

  socket.onopen = () => {
    console.log("Connected to room:", room);
    setConnected(true);
  };

  socket.onmessage = (event) => {
    console.log("Received:", event.data);
  };

  socket.onerror = (error) => {
    console.error("WebSocket error:", error);
    alert("WebSocket error. Check console.");
  };

  socket.onclose = () => {
    console.log("Disconnected");
    setConnected(false);
    socket = null;
  };
});

sendBtn.addEventListener("click", () => {
  const msg = msgInput.value.trim();
  if (!msg) return;
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(msg);
    console.log("Sent:", msg);
    msgInput.value = "";
  } else {
    alert("Not connected.");
  }
});

msgInput.addEventListener("keyup", (e) => {
  if (e.key === "Enter") sendBtn.click();
});

setConnected(false);
