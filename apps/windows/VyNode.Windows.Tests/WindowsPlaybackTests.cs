using Microsoft.VisualStudio.TestTools.UnitTesting;
using VyNode.Windows.Models;
using VyNode.Windows.Services;

namespace VyNode.Windows.Tests;

[TestClass]
public sealed class WindowsPlaybackTests
{
    [TestMethod]
    public void CapabilityProfileIsConservativeAndNative()
    {
        var profile = WindowsCapabilityService.Current();
        Assert.AreEqual("WINDOWS", profile.Platform);
        CollectionAssert.Contains(profile.SupportedContainers.ToList(), "mp4");
        CollectionAssert.Contains(profile.SupportedVideoCodecs.ToList(), "h264");
        CollectionAssert.DoesNotContain(profile.SupportedVideoCodecs.ToList(), "hevc");
        CollectionAssert.DoesNotContain(profile.SupportedVideoCodecs.ToList(), "av1");
    }

    [TestMethod]
    public void PlaybackRoutesKeepServerAndMediaIdentityTogether()
    {
        var user = new GlobalUser("user-id", "viewer", "Viewer");
        var server = new LinkedServer("server-id", "Server A", "MEMBER", [new ServerEndpoint("http://127.0.0.1:8096", "LOCAL", null)]);
        var context = new SessionContext(user, server, "http://127.0.0.1:8096", "access", LocalRole: "USER");
        var route = new PlaybackRoute(context, "EPISODE", "episode-2", "Episode Two", false, 0);
        Assert.AreEqual("server-id", route.Context.Server.Id);
        Assert.AreEqual("episode-2", route.LogicalId);
        Assert.IsFalse(route.Resume);
    }

    [TestMethod]
    public void WebVttCueIsParsedAtDeterministicTime()
    {
        var cue = WebVttParser.Parse("WEBVTT\n\n00:00:10.000 --> 00:00:15.000\nVyNode Subtitle Acceptance\n").Single();
        Assert.AreEqual(10, cue.Start);
        Assert.AreEqual(15, cue.End);
        Assert.AreEqual("VyNode Subtitle Acceptance", cue.Text);
    }
}
