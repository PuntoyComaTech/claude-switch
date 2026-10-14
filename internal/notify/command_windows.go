package notify

import "github.com/PuntoyComaTech/claude-switch/internal/sysexec"

// toast shows a Windows toast through the WinRT API built into PowerShell 5.
// The message is passed in an environment variable, never inside the script.
const toast = `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$n = $t.GetElementsByTagName('text')
$n.Item(0).AppendChild($t.CreateTextNode($env:CS_TITLE)) > $null
$n.Item(1).AppendChild($t.CreateTextNode($env:CS_MSG)) > $null
$id = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($id).Show([Windows.UI.Notifications.ToastNotification]::new($t))`

func command(msg string) sysexec.Cmd {
	return sysexec.Cmd{
		Name: "powershell.exe",
		Args: []string{"-NoProfile", "-NonInteractive", "-Command", toast},
		Env:  []string{"CS_TITLE=" + title, "CS_MSG=" + msg},
	}
}
