// OpenBowl Chrome Extension Content Script
// Injects compiled workspace contexts into ChatGPT and Claude inputs.

(function() {
  // Create floating Injector Button
  const btn = document.createElement('button');
  btn.innerHTML = '🥣 Inject Context';
  btn.style.position = 'fixed';
  btn.style.bottom = '20px';
  btn.style.right = '20px';
  btn.style.zIndex = '999999';
  btn.style.padding = '10px 16px';
  btn.style.borderRadius = '24px';
  btn.style.border = 'none';
  btn.style.backgroundColor = '#8b5cf6'; // OpenBowl purple
  btn.style.color = '#ffffff';
  btn.style.fontSize = '13px';
  btn.style.fontWeight = '600';
  btn.style.cursor = 'pointer';
  btn.style.boxShadow = '0 4px 14px rgba(139, 92, 246, 0.4)';
  btn.style.transition = 'transform 0.2s, background-color 0.2s';

  btn.onmouseover = () => {
    btn.style.backgroundColor = '#7c3aed';
    btn.style.transform = 'scale(1.05)';
  };
  btn.onmouseout = () => {
    btn.style.backgroundColor = '#8b5cf6';
    btn.style.transform = 'scale(1)';
  };

  btn.onclick = async () => {
    btn.innerHTML = '⏳ Loading...';
    
    // Read selected project ID from storage (fallback to default)
    chrome.storage.local.get(['projectId'], async (res) => {
      const projID = res.projectId || 'proj-core-default';
      
      try {
        const response = await fetch(`http://localhost:3010/api/v1/projects/${projID}/context`);
        if (!response.ok) throw new Error();
        const data = await response.json();
        const contextText = data.context_text;

        // Try to locate prompt box
        // ChatGPT selector: '#prompt-textarea'
        // Claude selector: 'div[contenteditable="true"]'
        let inputArea = document.querySelector('#prompt-textarea') || 
                        document.querySelector('div[contenteditable="true"]') ||
                        document.querySelector('textarea');

        if (inputArea) {
          if (inputArea.tagName === 'DIV') {
            // contenteditable element injection
            const prevHtml = inputArea.innerHTML;
            inputArea.focus();
            document.execCommand('insertText', false, contextText + "\n\n");
          } else {
            // textarea element injection
            const start = inputArea.selectionStart;
            const end = inputArea.selectionEnd;
            const text = inputArea.value;
            inputArea.value = text.substring(0, start) + contextText + "\n\n" + text.substring(end);
            inputArea.dispatchEvent(new Event('input', { bubbles: true }));
          }

          showToast('Bowl Context Injected!');
        } else {
          showToast('Error: Prompt input box not found!');
        }
      } catch (err) {
        showToast('Connection failed! Make sure OpenBowl is running.');
      } finally {
        btn.innerHTML = '🥣 Inject Context';
      }
    });
  };

  document.body.appendChild(btn);

  function showToast(message) {
    const toast = document.createElement('div');
    toast.innerHTML = message;
    toast.style.position = 'fixed';
    toast.style.bottom = '80px';
    toast.style.right = '20px';
    toast.style.zIndex = '999999';
    toast.style.padding = '8px 16px';
    toast.style.borderRadius = '8px';
    toast.style.backgroundColor = '#1f2937';
    toast.style.color = '#ffffff';
    toast.style.fontSize = '12px';
    toast.style.boxShadow = '0 4px 12px rgba(0,0,0,0.15)';
    toast.style.animation = 'fadeIn 0.2s';

    document.body.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 2500);
  }
})();
