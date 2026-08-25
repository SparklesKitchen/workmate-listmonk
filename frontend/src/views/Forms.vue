<template>
  <section class="forms content relative">
    <h1 class="title is-4">
      {{ $t('forms.title') }}
    </h1>
    <hr />

    <b-loading v-if="loading.lists" :active="loading.lists" :is-full-page="false" />
    <p v-else-if="publicLists.length === 0">
      {{ $t('forms.noPublicLists') }}
    </p>
    <div class="columns" v-else-if="publicLists.length > 0">
      <div class="column is-4">
        <h4>{{ $t('forms.publicLists') }}</h4>
        <p>{{ $t('forms.selectHelp') }}</p>

        <b-loading :active="loading.lists" :is-full-page="false" />
        <ul class="no" data-cy="lists">
          <li v-for="(l, i) in publicLists" :key="l.id">
            <b-checkbox v-model="checked" :native-value="i">
              {{ l.name }}
            </b-checkbox>
          </li>
        </ul>

        <template v-if="serverConfig.public_subscription.enabled">
          <hr />
          <h4>{{ $t('forms.publicSubPage') }}</h4>
          <p>
            <a :href="`${serverConfig.root_url}/subscription/form`" target="_blank" rel="noopener noreferer"
              data-cy="url">
              {{ serverConfig.root_url }}/subscription/form
            </a>
          </p>
        </template>

        <template v-if="!isWorkMateCustomer">
        <hr />
        <h4>{{ $t('forms.redirectURL') }}</h4>
        <p class="is-size-7 has-text-grey">
          {{ $t('forms.redirectURLHelp') }}
        </p>
        <ul v-if="redirectURLs.length > 0" class="no" data-cy="redirect-urls">
          <li>
            <b-radio v-model="selectedRedirectURL" native-value="">
              {{ $t('globals.terms.none') }}
            </b-radio>
          </li>
          <li v-for="url in redirectURLs" :key="url">
            <b-radio v-model="selectedRedirectURL" :native-value="url">
              {{ url }}
            </b-radio>
          </li>
        </ul>
        </template>
      </div>
      <div class="column" data-cy="form">
        <h4>{{ $t('forms.formHTML') }}</h4>
        <p>
          {{ $t('forms.formHTMLHelp') }}
        </p>

        <code-editor lang="html" v-if="checked.length > 0" v-model="html" disabled />
      </div>
    </div><!-- columns -->

    <hr />
    <div data-cy="form-designer">
      <h4>Form designer</h4>
      <p class="is-size-7 has-text-grey">
        Design a branded form for your own website. It carries no Reach or WorkMate branding
        and submits in place with your success message.
      </p>
      <b-button type="is-primary" class="mt-2" :disabled="checked.length === 0"
        data-cy="btn-open-designer" @click="isDesignerOpen = true">
        Open form designer
      </b-button>
      <p v-if="checked.length === 0" class="is-size-7 has-text-grey mt-2">
        Select at least one list above first.
      </p>
    </div>

    <b-modal v-model="isDesignerOpen" scroll="keep" :width="1100">
      <div class="box" style="position: relative;">
        <b-button size="is-small" style="position:absolute;top:14px;right:14px;"
          aria-label="Close" data-cy="btn-close-designer" @click="isDesignerOpen = false">&#215;</b-button>
        <h4>Form designer</h4>
        <div class="columns mt-2">
          <div class="column is-4">
            <b-field label="Heading"><b-input v-model="design.heading" /></b-field>
            <b-field label="Button text"><b-input v-model="design.button" /></b-field>
            <b-field><b-checkbox v-model="design.showName">Ask for the subscriber's name</b-checkbox></b-field>
            <b-field label="Consent text (optional)">
              <b-input v-model="design.consent" placeholder="I agree to receive this newsletter" />
            </b-field>
            <b-field label="Success message"><b-input v-model="design.success" /></b-field>
            <div class="columns">
              <div class="column"><b-field label="Background"><input type="color" v-model="design.bg" aria-label="Background color" /></b-field></div>
              <div class="column"><b-field label="Text"><input type="color" v-model="design.text" aria-label="Text color" /></b-field></div>
              <div class="column"><b-field label="Button"><input type="color" v-model="design.accent" aria-label="Button color" /></b-field></div>
            </div>
            <b-field label="Corner radius">
              <b-slider v-model="design.radius" :min="0" :max="24" />
            </b-field>
          </div>
          <div class="column is-4">
            <h5>Preview</h5>
            <iframe title="Form preview" :srcdoc="designedHTML" style="width:100%;height:440px;border:1px solid #ccc;background:#fff;" />
          </div>
          <div class="column is-4">
            <h5>Embed HTML</h5>
            <b-button size="is-small" class="mb-2" @click="copyDesignedHTML" data-cy="btn-copy-designed">Copy HTML</b-button>
            <code-editor lang="html" v-model="designedHTML" disabled />
          </div>
        </div>
      </div>
    </b-modal>
  </section>
</template>

<script>
import Vue from 'vue';
import { mapState } from 'vuex';
import CodeEditor from '../components/CodeEditor.vue';

export default Vue.extend({
  name: 'ListForm',

  components: {
    'code-editor': CodeEditor,
  },

  data() {
    return {
      checked: [],
      html: '',
      selectedRedirectURL: '',
      isDesignerOpen: false,
      design: {
        heading: 'Subscribe to our newsletter',
        button: 'Subscribe',
        showName: true,
        consent: '',
        success: 'Thanks! Please check your inbox to confirm.',
        bg: '#ffffff',
        text: '#1a1a2e',
        accent: '#0db7df',
        radius: 8,
      },
    };
  },

  methods: {
    escapeAttr(value) {
      return String(value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
    },

    copyDesignedHTML() {
      navigator.clipboard.writeText(this.designedHTML).then(() => {
        this.$utils.toast('Form HTML copied');
      });
    },

    renderHTML() {
      let h = `<form method="post" action="${this.serverConfig.root_url}/subscription/form" class="listmonk-form">\n`
        + '  <div>\n'
        + `    <h3>${this.$t('public.sub')}</h3>\n`
        + '    <input type="hidden" name="nonce" />\n';

      if (this.selectedRedirectURL) {
        h += `    <input type="hidden" name="next" value="${this.escapeAttr(this.selectedRedirectURL)}" />\n`;
      }

      h += '\n'
        + `    <p><input type="email" name="email" required placeholder="${this.$t('subscribers.email')}" /></p>\n`
        + `    <p><input type="text" name="name" placeholder="${this.$t('public.subName')}" /></p>\n\n`;

      this.checked.forEach((i) => {
        const l = this.publicLists[parseInt(i, 10)];

        h += '    <p>\n'
          + `      <input id="${l.uuid.substr(0, 5)}" type="checkbox" name="l" checked value="${l.uuid}" />\n`
          + `      <label for="${l.uuid.substr(0, 5)}">${l.name}</label>\n`;

        if (l.description) {
          h += '      <br />\n'
            + `      <span>${l.description}</span>\n`;
        }

        h += '    </p>\n';
      });

      // Captcha?
      if (this.serverConfig.public_subscription.captcha_enabled) {
        if (this.serverConfig.public_subscription.captcha_provider === 'altcha') {
          h += '\n'
            + `    <altcha-widget challengeurl="${this.serverConfig.root_url}/api/public/captcha/altcha"></altcha-widget>\n`
            + `    <${'script'} type="module" src="${this.serverConfig.root_url}/public/static/altcha.umd.js" async defer></${'script'}>\n`;
        } else if (this.serverConfig.public_subscription.captcha_provider === 'hcaptcha') {
          h += '\n'
            + `    <div class="h-captcha" data-sitekey="${this.serverConfig.public_subscription.captcha_key}"></div>\n`
            + `    <${'script'} src="https://js.hcaptcha.com/1/api.js" async defer></${'script'}>\n`;
        }
      }

      h += '\n'
        + `    <input type="submit" value="${this.$t('public.sub')} " />\n`
        + '  </div>\n'
        + '</form>';

      this.html = h;
    },
  },

  computed: {
    ...mapState(['loading', 'lists', 'serverConfig', 'profile']),

    isWorkMateCustomer() {
      return this.profile.userRole && this.profile.userRole.name === 'WorkMate Customer';
    },

    designedHTML() {
      const d = this.design;
      const esc = this.escapeAttr;
      const uuids = this.checked
        .map((i) => this.publicLists[parseInt(i, 10)])
        .filter((l) => l)
        .map((l) => l.uuid);
      const id = `nl-${uuids.length ? uuids[0].substr(0, 8) : 'form'}`;
      const root = this.serverConfig.root_url;
      const inputStyle = `display:block;width:100%;box-sizing:border-box;padding:10px 12px;margin:0 0 10px;border:1px solid #d0d5dd;border-radius:${d.radius}px;font:inherit;`;
      const consentStyle = 'display:flex;gap:8px;align-items:flex-start;font-size:13px;margin:0 0 12px;';
      const consent = d.consent
        ? `      <label style="${consentStyle}"><input type="checkbox" required style="margin-top:2px;" /> <span>${esc(d.consent)}</span></label>\n`
        : '';
      const nameField = d.showName
        ? `      <input type="text" name="name" placeholder="Name" style="${inputStyle}" />\n`
        : '';
      // ponytail: inline styles + one tiny script so the snippet works pasted anywhere
      return `<div id="${id}" style="background:${d.bg};color:${d.text};padding:24px;border-radius:${d.radius}px;max-width:420px;font-family:system-ui,sans-serif;">\n`
        + '  <form>\n'
        + `    <h3 style="margin:0 0 14px;font-size:19px;">${esc(d.heading)}</h3>\n`
        + `      <input type="email" name="email" required placeholder="E-mail" style="${inputStyle}" />\n${
          nameField
        }${consent
        }    <button type="submit" style="width:100%;padding:11px 0;border:0;border-radius:${d.radius}px;`
        + `background:${d.accent};color:#fff;font:600 15px system-ui,sans-serif;cursor:pointer;">${esc(d.button)}</button>\n`
        + '    <p data-nl-msg style="display:none;margin:12px 0 0;font-size:14px;"></p>\n'
        + '  </form>\n'
        + '</div>\n'
        + `<${'script'}>(function(){var w=document.getElementById("${id}"),f=w.querySelector("form"),m=w.querySelector("[data-nl-msg]");`
        + 'f.addEventListener("submit",function(e){e.preventDefault();'
        + `fetch("${root}/api/public/subscription",{method:"POST",headers:{"Content-Type":"application/json"},`
        + `body:JSON.stringify({email:f.email.value,name:f.name?f.name.value:"",list_uuids:${JSON.stringify(uuids)}})})`
        + `.then(function(r){m.style.display="block";if(r.ok){m.textContent="${esc(d.success)}";f.reset();}`
        + 'else{r.json().then(function(j){m.textContent=(j&&j.message)||"Something went wrong. Please try again.";});}})'
        + `.catch(function(){m.style.display="block";m.textContent="Something went wrong. Please try again.";});});})();</${'script'}>`;
    },

    publicLists() {
      if (!this.lists.results) {
        return [];
      }
      return this.lists.results.filter((l) => l.type === 'public');
    },

    redirectURLs() {
      const urls = this.serverConfig.public_subscription
        ? this.serverConfig.public_subscription.redirect_urls
        : [];
      return Array.isArray(urls) ? urls : [];
    },
  },

  watch: {
    checked() {
      this.renderHTML();
    },

    selectedRedirectURL() {
      this.renderHTML();
    },
  },
});
</script>
