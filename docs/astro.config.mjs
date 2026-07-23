// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import starlightBlog from "starlight-blog";
import starlightOGImages from "./src/routeData";

// https://astro.build/config
export default defineConfig({
  site: "https://tunnel.strrl.dev",
  integrations: [
    starlight({
      title: "Cloudflare Tunnel Ingress Controller",
      customCss: ["./src/styles/custom.css"],
      plugins: [
        starlightBlog({
          title: "Development Blog",
          prefix: "blog",
          authors: {
            strrl: {
              name: "STRRL",
              title: "Developer",
              url: "https://github.com/strrl",
              picture:
                "https://avatars.githubusercontent.com/u/20221408?s=400&u=6ba6413e865019ca18f4422e4c53fcb046ef0a8c&v=4",
            },
          },
        }),
        starlightOGImages(),
      ],
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/strrl/cloudflare-tunnel-ingress-controller",
        },
      ],
      sidebar: [
        {
          label: "Guides",
          items: [{ label: "Quickstart", slug: "guides/quickstart" }],
        },
        {
          label: "Reference",
          autogenerate: { directory: "reference" },
        },
      ],
      head: [
        {
          tag: "script",
          attrs: {
            src: "https://www.googletagmanager.com/gtag/js?id=G-CHHHFNJ6K5",
            async: true,
          },
        },
        {
          tag: "script",
          attrs: {
            type: "text/javascript",
          },
          content: `
            window.dataLayer = window.dataLayer || [];
            function gtag(){dataLayer.push(arguments);}
            gtag('js', new Date());
            gtag('config', 'G-CHHHFNJ6K5');
          `,
        },
        {
          tag: "script",
          attrs: {
            type: "text/javascript",
          },
          content: `
            (function(c,l,a,r,i,t,y){
                c[a]=c[a]||function(){(c[a].q=c[a].q||[]).push(arguments)};
                t=l.createElement(r);t.async=1;t.src="https://www.clarity.ms/tag/"+i;
                y=l.getElementsByTagName(r)[0];y.parentNode.insertBefore(t,y);
            })(window, document, "clarity", "script", "tx1nlf05gh");
          `,
        },
        {
          tag: "script",
          attrs: {
            type: "text/javascript",
          },
          content: `
            (function() {
              var img = document.createElement('img');
              img.referrerPolicy = 'no-referrer-when-downgrade';
              img.src = 'https://static.scarf.sh/a.png?x-pxid=8c03545e-5e36-4b2e-bb3b-d0e45626f71b';
              img.alt = '';
              img.style.display = 'none';
              img.width = 1;
              img.height = 1;
              document.body.appendChild(img);
            })();
          `,
        },
      ],
    }),
  ],
});
