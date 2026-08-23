<template>
  <section class="workmate-delivery">
    <header class="page-header">
      <h1 class="title is-4 mb-2">Delivery</h1>
      <p class="is-size-6 has-text-grey-light">
        Set the sender and connection for this WorkMate workspace. It is not shared with another workspace.
      </p>
    </header>

    <b-notification type="is-info" :closable="false">
      Use the SMTP credentials from Brevo or your email provider. The sender address must already be verified by that provider.
    </b-notification>

    <form class="box" @submit.prevent="save">
      <b-field label="Delivery provider">
        <b-select v-model="form.provider" expanded>
          <option value="brevo">Brevo SMTP</option>
          <option value="smtp">Other SMTP provider</option>
        </b-select>
      </b-field>

      <b-field label="Sender email" message="Campaigns from this workspace always use this sender.">
        <b-input v-model.trim="form.fromEmail" type="email" required />
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

      <b-field label="SMTP username">
        <b-input v-model.trim="form.username" required />
      </b-field>
      <b-field :label="delivery.configured ? 'New SMTP password or key (leave blank to keep it)' : 'SMTP password or key'">
        <b-input v-model="form.password" type="password" password-reveal :required="!delivery.configured" />
      </b-field>

      <b-button native-type="submit" type="is-primary" :loading="saving">
        Save delivery connection
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
      delivery: { configured: false },
      form: {
        provider: 'brevo',
        fromEmail: '',
        host: '',
        port: 587,
        username: '',
        password: '',
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
        this.delivery = { ...this.delivery, configured: true };
        this.form.password = '';
        await this.$root.awaitRestart(response);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>
