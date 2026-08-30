using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using VyNode.Windows.Auth;
using VyNode.Windows.Models;
using VyNode.Windows.Services;

namespace VyNode.Windows;

public sealed partial class SignInPage : Page
{
    private readonly ConnectClient _connect = new();
    private readonly DeviceIdentity _identity = new();
    private readonly SecureCredentialStore _credentials = new();
    private readonly LocalStateStore _state = new();
    private readonly SessionBootstrapper _bootstrapper = new();
    public SignInPage() => InitializeComponent();

    private async void SignIn_Click(object sender, RoutedEventArgs e)
    {
        var stage = "sign in";
        SetBusy(true);
        try
        {
            _identity.GetOrCreate();
            var device = DeviceIdentity.Describe();
            var login = await _connect.LoginAsync(UsernameBox.Text.Trim(), PasswordBox.Password, device, CancellationToken.None);
            stage = "secure credential storage";
            _credentials.SaveGlobal(login.User.Id, login.RefreshToken);
            await _state.SaveGlobalAsync(login.User, device);
            stage = "linked server discovery";
            var servers = await _connect.ServersAsync(login.AccessToken, CancellationToken.None);
            var global = new GlobalContext(login, servers, device);
            if (servers.Count == 0) { ((MainWindow)App.Window).ShowZeroServer(global); return; }
            if (servers.Count > 1) { ((MainWindow)App.Window).ShowServerSelector(global); return; }
            ((MainWindow)App.Window).ShowShell(await _bootstrapper.ConnectAsync(global, servers[0], CancellationToken.None));
        }
        catch (Exception ex) { ShowError($"Could not complete {stage}: {ex.Message}"); }
        finally { SetBusy(false); }
    }

    private void SetBusy(bool busy) { SignInButton.IsEnabled = !busy; SignInButton.Content = busy ? "Signing In…" : "Sign In"; }
    private void ShowError(string text) { ErrorText.Text = text; ErrorText.Visibility = Visibility.Visible; }
    private void CreateAccount_Click(object sender, RoutedEventArgs e) => ShowError("Account creation opens through the trusted VyNode Connect registration flow.");
    private void Advanced_Click(object sender, RoutedEventArgs e) => ((MainWindow)App.Window).ShowManualConnect();
}
