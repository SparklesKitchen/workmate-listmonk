<template>
  <section class="workmate-delivery">
    <header class="page-header">
      <h1 class="title is-4 mb-2">Delivery</h1>
      <p class="is-size-6 has-text-grey-light">
        Set the sender and connection for this WorkMate workspace. It is not shared with another workspace.
      </p>
    </header>

    <b-notification type="is-info" :closable="false">
      Save a provider, then send a verification email. Campaign tests, schedules, and sends stay blocked until the connection and sender verify.
    </b-notification>

    <b-notification v-if="delivery.configured && !delivery.verified" type="is-warning" :closable="false">
      This delivery connection is saved but unverified. It cannot send campaigns yet.
    </b-notification>

    <b-notification v-if="delivery.verified" type="is-success" :closable="false">
      This workspace delivery connection and sender are verified.
    </b-notification>

    <form class="box" @submit.prevent="save">
      <b-field label="Delivery provider">
        <b-select v-model="form.provider" expanded>
          <option value="brevo_api">Brevo API</option>
          <option value="brevo_smtp">Brevo SMTP relay</option>
          <option value="smtp">Generic SMTP</option>
        </b-select>
      </b-field>

      <b-field label="Sender email" message="Campaigns from this workspace always use this sender.">
        <b-input v-model.trim="form.fromEmail" type="email" required />
      </b-field>

      <b-field v-if="form.provider === 'brevo_api'" label="Sender name">
        <b-input v-model.trim="form.senderName" required />
      </b-field>

      <template v-if="form.provider === 'smtp'">
        <div class="columns">
          <div class="column">
            <b-field label="SMTP host">
              <b-input v-model.trim="form.host" required />
            </b-field>
          </div>
          <div class="column is-3">
            <b-field label="Port">
              <b-input v-model.number="form.port" type="number" min="1" max="65535" required />
            </b-field>
          </div>
        </div>

        <div class="columns">
          <div class="column">
            <b-field label="Authentication">
              <b-select v-model="form.authProtocol" expanded>
                <option value="login">LOGIN</option>
                <option value="plain">PLAIN</option>
                <option value="cram">CRAM-MD5</option>
                <option value="none">None</option>
              </b-select>
            </b-field>
          </div>
          <div class="column">
            <b-field label="TLS">
              <b-select v-model="form.tlsType" expanded>
                <option value="STARTTLS">STARTTLS</option>
                <option value="TLS">TLS</option>
                <option value="none">None</option>
              </b-select>
            </b-field>
          </div>
        </div>
      </template>

      <template v-if="form.provider === 'brevo_smtp'">
        <b-field label="Brevo SMTP relay">
          <b-input value="smtp-relay.brevo.com" disabled />
        </b-field>
        <b-field label="SMTP port">
          <b-input v-model.number="form.port" type="number" min="1" max="65535" required />
        </b-field>
      </template>

      <b-field v-if="form.provider !== 'brevo_api'" label="SMTP username">
        <b-input v-model.trim="form.username" required />
      </b-field>
      <b-field :label="credentialLabel">
        <b-input v-model="form.password" type="password" password-reveal :required="!delivery.configured" />
      </b-field>

      <b-button native-type="submit" type="is-primary" :loading="saving">
        Save delivery connection
      </b-button>
    </form>

    <form v-if="delivery.configured && !delivery.verified" class="box verify-box" @submit.prevent="verify">
      <h2 class="title is-5">Verify delivery</h2>
      <p class="mb-4">Send a verification email before using this connection for campaign tests, schedules, or sends.</p>
      <b-field label="Test recipient email">
        <b-input v-model.trim="testEmail" type="email" required />
      </b-field>
      <b-button native-type="submit" type="is-primary" :loading="verifying">
        Send verification email
      </b-button>
    </form>
  </section>
</template>

<script>
export default {
  name: 'WorkMateDelivery',

  data() {
    return {
      saving: false,
      verifying: false,
      testEmail: '',
      delivery: { configured: false, verified: false },
      form: {
        provider: 'brevo_api',
        fromEmail: '',
        host: '',
        port: 587,
        username: '',
        password: '',
        senderName: '',
        authProtocol: 'login',
        tlsType: 'STARTTLS',
      },
    };
  },

  async mounted() {
    const delivery = await this.$api.getWorkMateDelivery();
    this.delivery = delivery;
    if (delivery.configured) {
      this.form = {
        ...this.form,
        provider: delivery.provider || 'smtp',
        senderName: delivery.senderName || '',
        fromEmail: delivery.fromEmail,
        host: delivery.host,
        port: delivery.port,
        username: delivery.username,
        authProtocol: delivery.authProtocol,
        tlsType: delivery.tlsType,
      };
    }
  },

  methods: {
    async save() {
      this.saving = true;
      try {
        const response = await this.$api.updateWorkMateDelivery(this.form);
        this.delivery = {
          ...this.delivery,
          configured: true,
          verified: false,
          provider: this.form.provider,
        };
        this.form.password = '';
        await this.$root.awaitRestart(response);
      } finally {
        this.saving = false;
      }
    },
    async verify() {
      this.verifying = true;
      try {
        const response = await this.$api.verifyWorkMateDelivery({
          provider: this.form.provider,
          test_email: this.testEmail,
        });
        this.delivery = { ...this.delivery, verified: true };
        await this.$root.awaitRestart(response);
      } finally {
        this.verifying = false;
      }
    },
  },
  computed: {
    credentialLabel() {
      const kind = this.form.provider === 'brevo_api' ? 'Brevo API key' : 'SMTP password or key';
      return this.delivery.configured ? `New ${kind} (leave blank to keep it)` : kind;
    },
  },
};
</script>
