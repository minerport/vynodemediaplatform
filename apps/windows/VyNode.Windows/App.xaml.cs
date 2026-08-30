using Microsoft.UI.Xaml;
using VyNode.Windows.Services;

namespace VyNode.Windows;

public partial class App : Application
{
    public static MainWindow Window { get; private set; } = null!;
    public App() => InitializeComponent();
    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        ApplicationRestartRegistration.Register();
        Window = new MainWindow();
        Window.Activate();
    }
}
