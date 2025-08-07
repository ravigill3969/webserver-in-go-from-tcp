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

  sessionStorage.setItem("room", room);

  // Redirect to chat page

  if (!room) {
    alert("Please enter a room key.");
    return;
  }

  window.location.assign("/chat/chat.html");
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
