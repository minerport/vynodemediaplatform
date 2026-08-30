using System.Net;
using System.Security.Cryptography.X509Certificates;
using Microsoft.AspNetCore.StaticFiles;

var root = Environment.GetEnvironmentVariable("VYNODE_ACCEPTANCE_FEED_ROOT")
    ?? throw new InvalidOperationException("VYNODE_ACCEPTANCE_FEED_ROOT is required.");
var thumbprint = Environment.GetEnvironmentVariable("VYNODE_ACCEPTANCE_CERT_THUMBPRINT")
    ?? throw new InvalidOperationException("VYNODE_ACCEPTANCE_CERT_THUMBPRINT is required.");
using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
store.Open(OpenFlags.ReadOnly);
var certificate = store.Certificates.Find(X509FindType.FindByThumbprint, thumbprint, validOnly: true).OfType<X509Certificate2>().Single();

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.ConfigureKestrel(options => options.Listen(IPAddress.Loopback, 18443, endpoint => endpoint.UseHttps(certificate)));
var app = builder.Build();
var contentTypes = new FileExtensionContentTypeProvider();
contentTypes.Mappings[".sig"] = "application/octet-stream";
app.UseStaticFiles(new StaticFileOptions
{
    FileProvider = new Microsoft.Extensions.FileProviders.PhysicalFileProvider(Path.GetFullPath(root)),
    ContentTypeProvider = contentTypes
});
app.Run();
