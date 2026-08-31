package notification

import "html/template"

// These table-based templates deliberately use inline styles and bgcolor attributes.
// That keeps the Social Fund palette intact in Gmail, Outlook, and mobile mail clients.
var styledAccountHTMLTemplate = template.Must(template.New("styled-account-created").Parse(`<!doctype html>
<html><head><meta name="color-scheme" content="light"><meta name="supported-color-schemes" content="light"></head>
<body bgcolor="#EAE0CF" style="margin:0;padding:0;background-color:#EAE0CF;color:#213448;font-family:Arial,sans-serif">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" bgcolor="#EAE0CF" style="background-color:#EAE0CF"><tr><td align="center" style="padding:36px 14px">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" bgcolor="#EAE0CF" style="width:100%;max-width:600px;background-color:#EAE0CF;border-radius:12px;overflow:hidden;box-shadow:0 18px 44px #94B4C1">
<tr><td bgcolor="#213448" style="background-color:#213448;padding:25px 30px;color:#EAE0CF"><div style="font-size:22px;font-weight:700">Social Fund</div><div style="margin-top:4px;color:#94B4C1;font-size:12px">Community finance</div></td></tr>
<tr><td style="padding:32px 30px">
<span style="display:inline-block;background-color:#94B4C1;color:#213448;border-radius:6px;padding:7px 10px;font-size:11px;font-weight:700;letter-spacing:1px">ACCOUNT CREATED</span>
<h1 style="margin:20px 0 8px;color:#213448;font-size:25px;line-height:1.25">Welcome, {{.FullName}}</h1>
<p style="margin:0 0 26px;color:#547792;font-size:15px;line-height:1.65">Your Social Fund account is ready. Review your details and activate it using your registered Google account.</p>
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" bgcolor="#94B4C1" style="background-color:#94B4C1;border-radius:9px;color:#213448">
<tr><td style="padding:16px 18px 8px;font-size:13px;font-weight:700">Account details</td></tr>
<tr><td style="padding:8px 18px;color:#547792;font-size:12px">Full name</td><td style="padding:8px 18px;text-align:right;font-size:13px;font-weight:700">{{.FullName}}</td></tr>
<tr><td style="padding:8px 18px;color:#547792;font-size:12px">Email</td><td style="padding:8px 18px;text-align:right;font-size:13px">{{.Email}}</td></tr>
<tr><td style="padding:8px 18px;color:#547792;font-size:12px">Phone</td><td style="padding:8px 18px;text-align:right;font-size:13px">{{.Phone}}</td></tr>
<tr><td style="padding:8px 18px;color:#547792;font-size:12px">Contribution</td><td style="padding:8px 18px;text-align:right;font-size:13px;font-weight:700">{{.ContributionAmount}}</td></tr>
<tr><td style="padding:8px 18px;color:#547792;font-size:12px">Frequency</td><td style="padding:8px 18px;text-align:right;font-size:13px">{{.ContributionFrequency}}</td></tr>
<tr><td style="padding:8px 18px 16px;color:#547792;font-size:12px">Payment due</td><td style="padding:8px 18px 16px;text-align:right;font-size:13px">{{.PaymentDue}}</td></tr>
</table>
<p style="margin:24px 0 8px;color:#213448;font-size:14px;font-weight:700">Your account is currently inactive.</p>
<p style="margin:0;color:#547792;font-size:14px;line-height:1.6">Sign in as {{.Email}} to verify and activate your membership.</p>
<table role="presentation" cellspacing="0" cellpadding="0" style="margin-top:26px"><tr><td bgcolor="#547792" style="background-color:#547792;border-radius:8px"><a href="{{.LoginURL}}" style="display:inline-block;padding:13px 22px;color:#EAE0CF;text-decoration:none;font-size:14px;font-weight:700">Login to Social Fund</a></td></tr></table>
</td></tr>
<tr><td bgcolor="#94B4C1" style="background-color:#94B4C1;padding:18px 30px;color:#213448;font-size:11px;line-height:1.5">Automated message from Social Fund. Keep this email for your records.</td></tr>
</table></td></tr></table></body></html>`))

var styledNotificationHTMLTemplate = template.Must(template.New("styled-notification").Parse(`<!doctype html>
<html><head><meta name="color-scheme" content="light"><meta name="supported-color-schemes" content="light"></head>
<body bgcolor="#EAE0CF" style="margin:0;padding:0;background-color:#EAE0CF;color:#213448;font-family:Arial,sans-serif">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" bgcolor="#EAE0CF"><tr><td align="center" style="padding:36px 14px">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" bgcolor="#EAE0CF" style="width:100%;max-width:600px;background-color:#EAE0CF;border-radius:12px;overflow:hidden;box-shadow:0 18px 44px #94B4C1">
<tr><td bgcolor="#213448" style="background-color:#213448;padding:25px 30px;color:#EAE0CF"><div style="font-size:22px;font-weight:700">Social Fund</div><div style="margin-top:4px;color:#94B4C1;font-size:12px">Community finance</div></td></tr>
<tr><td style="padding:32px 30px">
<span style="display:inline-block;background-color:{{.Tint}};color:{{.Accent}};border-radius:6px;padding:7px 10px;font-size:11px;font-weight:700;letter-spacing:1px">{{.Label}}</span>
<h1 style="margin:20px 0;color:#213448;font-size:24px;line-height:1.3">{{.Subject}}</h1>
<div style="background-color:#94B4C1;color:#213448;border-radius:9px;padding:20px;font-size:15px;line-height:1.7">{{.Body}}</div>
{{if .ProofURL}}<table role="presentation" cellspacing="0" cellpadding="0" style="margin-top:24px"><tr><td bgcolor="#547792" style="background-color:#547792;border-radius:8px"><a href="{{.ProofURL}}" style="display:inline-block;padding:12px 18px;color:#EAE0CF;text-decoration:none;font-size:14px;font-weight:700">View payment proof</a></td></tr></table>{{end}}
{{if .ApproveURL}}<table role="presentation" cellspacing="0" cellpadding="0" style="margin-top:16px"><tr><td bgcolor="#547792" style="background-color:#547792;border-radius:8px"><a href="{{.ApproveURL}}" style="display:inline-block;padding:12px 20px;color:#EAE0CF;text-decoration:none;font-weight:700">Approve</a></td><td style="width:10px"></td><td bgcolor="#213448" style="background-color:#213448;border-radius:8px"><a href="{{.RejectURL}}" style="display:inline-block;padding:12px 20px;color:#EAE0CF;text-decoration:none;font-weight:700">Reject</a></td></tr></table>{{end}}
</td></tr>
<tr><td bgcolor="#94B4C1" style="background-color:#94B4C1;padding:18px 30px;color:#213448;font-size:11px;line-height:1.5">Automated message from Social Fund. Keep transaction references for your records.</td></tr>
</table></td></tr></table></body></html>`))
