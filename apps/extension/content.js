// OpenBowl Chrome Extension Content Script
// Injects compiled workspace contexts and automatically syncs conversations.

(function () {
  // Create floating Injector Button
  const btn = document.createElement("button");
  btn.id = "openbowl-btn";
  btn.innerHTML = "🥣 Inject Context";
  btn.style.position = "fixed";
  btn.style.bottom = "20px";
  btn.style.right = "20px";
  btn.style.zIndex = "999999";
  btn.style.padding = "10px 16px";
  btn.style.borderRadius = "24px";
  btn.style.border = "none";
  btn.style.backgroundColor = "#8b5cf6"; // OpenBowl purple
  btn.style.color = "#ffffff";
  btn.style.fontSize = "13px";
  btn.style.fontWeight = "600";
  btn.style.cursor = "pointer";
  btn.style.boxShadow = "0 4px 14px rgba(139, 92, 246, 0.4)";
  btn.style.transition = "transform 0.2s, background-color 0.2s";

  btn.onmouseover = () => {
    btn.style.backgroundColor = "#7c3aed";
    btn.style.transform = "scale(1.05)";
  };
  btn.onmouseout = () => {
    btn.style.backgroundColor = "#8b5cf6";
    btn.style.transform = "scale(1)";
  };

  btn.onclick = async () => {
    btn.innerHTML = "⏳ Loading...";

    // Read selected project ID from storage (fallback to default)
    chrome.storage.local.get(["projectId"], async (res) => {
      const projID = res.projectId || "proj-core-default";

      try {
        // 1. Force a manual sync of the current page messages before fetching context
        await forceSync(projID);

        // 2. Fetch compiled context (now containing recently synced message history)
        const response = await fetch(
          `http://localhost:3010/api/v1/projects/${projID}/context`,
        );
        if (!response.ok) throw new Error();
        const data = await response.json();
        const contextText = data.context_text;

        // Try to locate prompt box
        let inputArea =
          document.querySelector("#prompt-textarea") ||
          document.querySelector('div[contenteditable="true"]') ||
          document.querySelector("textarea");

        if (inputArea) {
          if (inputArea.tagName === "DIV") {
            // contenteditable element injection
            inputArea.focus();
            document.execCommand("insertText", false, contextText + "\n\n");
          } else {
            // textarea element injection
            const start = inputArea.selectionStart;
            const end = inputArea.selectionEnd;
            const text = inputArea.value;
            inputArea.value =
              text.substring(0, start) +
              contextText +
              "\n\n" +
              text.substring(end);
            inputArea.dispatchEvent(new Event("input", { bubbles: true }));
          }

          showToast("Bowl Context Injected!");
        } else {
          showToast("Error: Prompt input box not found!");
        }
      } catch (err) {
        showToast("Connection failed! Make sure OpenBowl is running.");
      } finally {
        btn.innerHTML = "🥣 Inject Context";
      }
    });
  };

  document.body.appendChild(btn);

  // Auto-sync state variables
  let lastSyncedText = "";
  let syncTimeout = null;

  // Scrapes active chat dialogs from the webpage DOM
  function scrapeMessages() {
    const messages = [];

    // 1. ChatGPT Selector (standard MV3 chat turns)
    const chatgptTurns = document.querySelectorAll(
      "[data-message-author-role]",
    );
    if (chatgptTurns.length > 0) {
      chatgptTurns.forEach((el) => {
        const role = el.getAttribute("data-message-author-role");
        const textEl =
          el.querySelector(".markdown") ||
          el.querySelector(".whitespace-pre-wrap") ||
          el;
        const content = textEl.innerText || textEl.textContent || "";
        if (content.trim() && (role === "user" || role === "assistant")) {
          messages.push({ role, content: content.trim() });
        }
      });
      return messages;
    }

    // 2. Claude Selector (both standard web and Playwright test selectors)
    const claudeTurns = document.querySelectorAll(
      'div[data-testid="user-message"], div[data-testid="assistant-message"], .font-claude-message, .claude-message',
    );
    if (claudeTurns.length > 0) {
      claudeTurns.forEach((el) => {
        let role = "user";
        if (
          el.classList.contains("font-claude-message") ||
          el.classList.contains("claude-message") ||
          el.getAttribute("data-testid") === "assistant-message"
        ) {
          role = "assistant";
        }
        const content = el.innerText || el.textContent || "";
        if (content.trim()) {
          messages.push({ role, content: content.trim() });
        }
      });
      return messages;
    }

    // 3. Fallback Selector for custom local model runtimes
    const genericTurns = document.querySelectorAll(
      ".user-message, .assistant-message, .chat-message",
    );
    genericTurns.forEach((el) => {
      let role = el.classList.contains("assistant-message")
        ? "assistant"
        : "user";
      const content = el.innerText || el.textContent || "";
      if (content.trim()) {
        messages.push({ role, content: content.trim() });
      }
    });

    return messages;
  }

  // Force synchronous push to backend database
  async function forceSync(projID) {
    const messages = scrapeMessages();
    if (messages.length === 0) return;

    const serialized = JSON.stringify(messages);
    try {
      await fetch("http://localhost:3010/api/v1/conversations/sync", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ project_id: projID, messages }),
      });
      lastSyncedText = serialized;
    } catch (e) {
      console.warn("OpenBowl: Manual sync failed", e);
    }
  }

  // Automatically sends messages in background as you chat
  async function autoSync() {
    chrome.storage.local.get(["projectId"], async (res) => {
      const projID = res.projectId || "proj-core-default";
      const messages = scrapeMessages();
      if (messages.length === 0) return;

      const serialized = JSON.stringify(messages);
      if (serialized === lastSyncedText) return; // Already synchronized

      try {
        await fetch("http://localhost:3010/api/v1/conversations/sync", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ project_id: projID, messages }),
        });
        lastSyncedText = serialized;
        console.log("OpenBowl: Conversation auto-synced successfully");
      } catch (e) {
        console.warn("OpenBowl: Background auto-sync failed", e);
      }
    });
  }

  // Listens to DOM changes to trigger background sync when messages are added
  function setupAutoSync() {
    const debouncedSync = () => {
      clearTimeout(syncTimeout);
      syncTimeout = setTimeout(autoSync, 2000); // Trigger 2 seconds after user/assistant stops typing
    };

    const observer = new MutationObserver((mutations) => {
      let shouldTrigger = false;
      for (const mutation of mutations) {
        // Skip mutations caused by the extension UI itself
        if (
          mutation.target.closest &&
          (mutation.target.closest("#openbowl-btn") ||
            mutation.target.closest(".openbowl-toast"))
        ) {
          continue;
        }
        shouldTrigger = true;
      }
      if (shouldTrigger) {
        debouncedSync();
      }
    });

    observer.observe(document.body, {
      childList: true,
      subtree: true,
    });
  }

  // Start background auto-sync observer
  setupAutoSync();

  function showToast(message) {
    const toast = document.createElement("div");
    toast.className = "openbowl-toast";
    toast.innerHTML = message;
    toast.style.position = "fixed";
    toast.style.bottom = "80px";
    toast.style.right = "20px";
    toast.style.zIndex = "999999";
    toast.style.padding = "8px 16px";
    toast.style.borderRadius = "8px";
    toast.style.backgroundColor = "#1f2937";
    toast.style.color = "#ffffff";
    toast.style.fontSize = "12px";
    toast.style.boxShadow = "0 4px 12px rgba(0,0,0,0.15)";
    toast.style.animation = "fadeIn 0.2s";

    document.body.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 2500);
  }
})();
