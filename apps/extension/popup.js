// OpenBowl Popup Controller script

document.addEventListener("DOMContentLoaded", () => {
  const projInput = document.getElementById("proj-id");
  const saveBtn = document.getElementById("save-btn");
  const statusText = document.getElementById("conn-text");

  // 1. Load current settings
  chrome.storage.local.get(["projectId"], (res) => {
    if (res.projectId) {
      projInput.value = res.projectId;
    } else {
      projInput.value = "proj-core-default";
    }
  });

  // 2. Ping Go Sidecar Server status
  fetch("http://localhost:3010/api/v1/health")
    .then((res) => {
      if (res.ok) {
        statusText.innerHTML = '<span class="dot dot-online"></span>Connected';
      } else {
        throw new Error();
      }
    })
    .catch(() => {
      statusText.innerHTML =
        '<span class="dot dot-offline"></span>Disconnected';
    });

  // 3. Save configs handler
  saveBtn.addEventListener("click", () => {
    const val = projInput.value.trim();
    chrome.storage.local.set({ projectId: val }, () => {
      saveBtn.innerText = "Saved!";
      setTimeout(() => {
        saveBtn.innerText = "Save Configurations";
      }, 1500);
    });
  });
});
