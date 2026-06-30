import * as vscode from "vscode";
import * as http from "http";

export function activate(context: vscode.ExtensionContext) {
  console.log("OpenBowl VS Code extension active!");

  let disposable = vscode.commands.registerCommand(
    "openbowl.injectContext",
    async () => {
      const config = vscode.workspace.getConfiguration("openbowl");
      const sidecarUrl =
        config.get<string>("sidecarUrl") || "http://localhost:3010";
      const projectId = config.get<string>("projectId") || "proj-core-default";

      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        vscode.window.showInformationMessage(
          "Open a text file first to inject workspace context.",
        );
        return;
      }

      vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Fetching OpenBowl Workspace Context...",
          cancellable: false,
        },
        async () => {
          try {
            const contextText = await fetchContext(sidecarUrl, projectId);

            // Inject fetched context text at active cursor location
            editor.edit((editBuilder) => {
              const position = editor.selection.active;
              editBuilder.insert(position, contextText);
            });

            vscode.window.showInformationMessage(
              "OpenBowl context successfully injected!",
            );
          } catch (err: any) {
            vscode.window.showErrorMessage(
              `OpenBowl: Connection failed. Verify sidecar is running at ${sidecarUrl}. Error: ${err.message}`,
            );
          }
        },
      );
    },
  );

  context.subscriptions.push(disposable);
}

function fetchContext(baseUrl: string, projectId: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const url = `${baseUrl}/api/v1/projects/${projectId}/context`;
    http
      .get(url, (res) => {
        if (res.statusCode !== 200) {
          reject(new Error(`Server returned code: ${res.statusCode}`));
          return;
        }

        let rawData = "";
        res.on("data", (chunk) => {
          rawData += chunk;
        });
        res.on("end", () => {
          try {
            const parsed = JSON.parse(rawData);
            if (parsed.context_text) {
              resolve(parsed.context_text);
            } else {
              reject(new Error("No context_text property returned by server."));
            }
          } catch (e) {
            reject(e);
          }
        });
      })
      .on("error", (e) => {
        reject(e);
      });
  });
}

export function deactivate() {}
