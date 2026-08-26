# Analytics (optional)

Self-hosted [GoatCounter](https://www.goatcounter.com/) is optional. The install Analytics gate only scaffolds config keys; finish token / public URL setup in [deploy/README.md](https://github.com/newtosh/nitpub/blob/main/deploy/README.md#goatcounter-analytics-optional).

Once configured, Admin → Analytics shows pageviews, top pages, top referrers, and top locations for the last 24 hours / 7 days / 30 days — proxied server-side so the GoatCounter token never reaches the browser. Internal traffic (`/admin`, `/login`, `/logout`, `/verify-*`) is tagged **self** rather than hidden, and a referrer with no `Referer` header reads as "Direct / no referrer" instead of a blank row.

![Analytics dashboard](/images/analytics-dashboard.png)
