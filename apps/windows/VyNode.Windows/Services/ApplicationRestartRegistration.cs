using System.Runtime.InteropServices;

namespace VyNode.Windows.Services;

internal static class ApplicationRestartRegistration
{
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    private static extern int RegisterApplicationRestart(string? commandLine, uint flags);

    public static void Register()
    {
        // A null command line restarts the installed executable without carrying
        // credentials or other process arguments across the installer boundary.
        _ = RegisterApplicationRestart(null, 0);
    }
}
