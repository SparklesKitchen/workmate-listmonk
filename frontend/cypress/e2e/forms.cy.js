const apiUrl = Cypress.env('apiUrl');

describe('Forms', () => {
  it('Opens forms page', () => {
    cy.resetDB();
    cy.loginAndVisit('/admin/lists/forms');
  });

  it('Checks public lists', () => {
    cy.get('ul[data-cy=lists] li')
      .should('contain', 'Opt-in list')
      .its('length')
      .should('eq', 1);

    cy.get('[data-cy=form] [role=textbox]').should('not.exist');
  });

  it('Selects public list', () => {
    // Click the list checkbox.
    cy.get('ul[data-cy=lists] .checkbox').click();

    // Check that the ID of the list in the checkbox appears in the HTML.
    cy.get('ul[data-cy=lists] input').then(($inp) => {
      cy.get('[data-cy=btn-show-form-html]').click();
      cy.get('[data-cy=form] [role=textbox]').contains($inp.val());
    });

    // Click the list checkbox.
    cy.get('ul[data-cy=lists] .checkbox').click();
    cy.get('[data-cy=form] pre').should('not.exist');
  });

  it('Subscribes from public form page', () => {
    // Create a public test list.
    cy.request('POST', `${apiUrl}/api/lists`, { name: 'test-list', type: 'public', optin: 'single' });

    // Open the public page and subscribe to alternating lists multiple times.
    // There should be no errors and two new subscribers should be subscribed to two lists.
    for (let i = 0; i < 2; i++) {
      for (let j = 0; j < 2; j++) {
        cy.loginAndVisit(`${apiUrl}/subscription/form`);
        cy.get('input[name=email]').clear().type(`test${i}@test.com`);
        cy.get('input[name=name]').clear().type(`test${i}`);
        cy.get('input[type=checkbox]').eq(j).click();
        cy.get('button').click();
        cy.wait(250);
        cy.get('.wrap').contains(/has been sent|successfully|retry/); // If SMTP is not configured, it shows retry message.
      }
    }

    // Verify form subscriptions.
    cy.request(`${apiUrl}/api/subscribers`).should((response) => {
      const { data } = response.body;

      // Two new + two dummy subscribers that are there by default.
      expect(data.total).to.equal(4);

      // The two new subscribers should each have two list subscriptions.
      for (let i = 0; i < 2; i++) {
        expect(data.results.find((s) => s.email === `test${i}@test.com`).lists.length).to.equal(2);
      }
    });
  });

  it('Saves ordered fields and submits their answers', () => {
    let listUUID;
    cy.request(`${apiUrl}/api/lists`).then((response) => {
      listUUID = response.body.data.results.find((list) => list.name === 'Opt-in list').uuid;
    });
    cy.get('[data-cy=lists] .checkbox').first().click();
    cy.get('[data-cy=btn-open-designer]').click();
    cy.get('[data-cy=btn-add-field]').click();
    cy.get('[data-cy=btn-add-field]').click();
    cy.get('[data-cy=designer-field]').first().within(() => {
      cy.get('input').first().clear().type('Company');
    });
    cy.get('[data-cy=designer-field]').eq(1).within(() => {
      cy.get('input').first().clear().type('Department');
    });
    cy.get('[data-cy=designer-field]').eq(1).trigger('dragstart');
    cy.get('[data-cy=designer-field]').first().trigger('drop');
    cy.get('[data-cy=btn-save-designer]').click();
    cy.get('[data-cy=btn-close-designer]').click();
    cy.reload();
    cy.get('[data-cy=lists] .checkbox').first().click();
    cy.get('[data-cy=btn-open-designer]').click();
    cy.get('[data-cy=designer-field]').first().should('contain', 'Department');
    cy.get('[data-cy=form-designer]').should('contain', 'name="attribs.field_1"');
    cy.get('[data-cy=btn-close-designer]').click();
    cy.loginAndVisit(`${apiUrl}/subscription/form?l=${listUUID}`);
    cy.get('input[name=email]').type('fields@test.com');
    cy.get('input[name="attribs.field_1"]').type('WorkMate');
    cy.get('button[type=submit]').click();
    cy.request(`${apiUrl}/api/subscribers`).its('body.data.results').should((subs) => {
      expect(subs.find((sub) => sub.email === 'fields@test.com').attribs.field_1).to.equal('WorkMate');
    });
  });

  it('Keeps hosted form schemas with their selected public list', () => {
    let first;
    let second;
    cy.request('POST', `${apiUrl}/api/lists`, { name: 'per-list-first', type: 'public', optin: 'single' }).then((response) => {
      first = response.body.data;
    });
    cy.request('POST', `${apiUrl}/api/lists`, { name: 'per-list-second', type: 'public', optin: 'single' }).then((response) => {
      second = response.body.data;
    });
    cy.reload();

    cy.contains('ul[data-cy=lists] label', 'per-list-first').click();
    cy.get('[data-cy=btn-open-designer]').click();
    cy.get('[data-cy=btn-add-field]').click();
    cy.get('[data-cy=designer-field]').first().find('input').first().clear().type('First company');
    cy.get('[data-cy=btn-save-designer]').click();
    cy.get('[data-cy=btn-close-designer]').click();
    cy.contains('ul[data-cy=lists] label', 'per-list-first').click();

    cy.contains('ul[data-cy=lists] label', 'per-list-second').click();
    cy.get('[data-cy=btn-open-designer]').click();
    cy.get('[data-cy=btn-add-field]').click();
    cy.get('[data-cy=designer-field]').first().find('input').first().clear().type('Second company');
    cy.get('[data-cy=btn-save-designer]').click();
    cy.get('[data-cy=btn-close-designer]').click();

    cy.contains('ul[data-cy=lists] label', 'per-list-second').click();
    cy.contains('ul[data-cy=lists] label', 'per-list-first').click();
    cy.get('[data-cy=btn-open-designer]').click();
    cy.get('[data-cy=form-designer]').should('contain', 'First company');
    cy.get('[data-cy=form-designer]').should('contain', `list_uuids:["${first.uuid}"]`);
    cy.get('[data-cy=btn-close-designer]').click();
    cy.get('[data-cy=url]').should('have.attr', 'href', `${apiUrl}/subscription/form?l=${first.uuid}`);
    cy.loginAndVisit(`${apiUrl}/subscription/form?l=${first.uuid}`);
    cy.get('label').should('contain', 'First company');

    cy.loginAndVisit('/admin/lists/forms');
    cy.contains('ul[data-cy=lists] label', 'per-list-second').click();
    cy.get('[data-cy=btn-open-designer]').click();
    cy.get('[data-cy=form-designer]').should('contain', 'Second company');
    cy.get('[data-cy=form-designer]').should('contain', `list_uuids:["${second.uuid}"]`);
    cy.get('[data-cy=btn-close-designer]').click();
    cy.get('[data-cy=url]').should('have.attr', 'href', `${apiUrl}/subscription/form?l=${second.uuid}`);
    cy.loginAndVisit(`${apiUrl}/subscription/form?l=${second.uuid}`);
    cy.get('label').should('contain', 'Second company');
    cy.get('label').should('not.contain', 'First company');
  });

  it('Unsubscribes', () => {
    // Add all lists to the dummy campaign.
    cy.request('PUT', `${apiUrl}/api/campaigns/1`, { lists: [2] });

    cy.request('GET', `${apiUrl}/api/subscribers`).then((response) => {
      const subUUID = response.body.data.results[0].uuid;

      cy.request('GET', `${apiUrl}/api/campaigns`).then((response) => {
        const campUUID = response.body.data.results[0].uuid;
        cy.loginAndVisit(`${apiUrl}/subscription/${campUUID}/${subUUID}`);
      });
    });

    cy.wait(500);

    // Unsubscribe from one list.
    cy.get('button').click();
    cy.request('GET', `${apiUrl}/api/subscribers`).then((response) => {
      const { data } = response.body;
      expect(data.results[0].lists.find((s) => s.id === 2).subscription_status).to.equal('unsubscribed');
      expect(data.results[0].lists.find((s) => s.id === 3).subscription_status).to.equal('unconfirmed');
    });

    // Go back.
    cy.url().then((u) => {
      cy.loginAndVisit(u);
    });

    // Unsubscribe from all.
    cy.get('#privacy-blocklist').click();
    cy.get('button').click();

    cy.request('GET', `${apiUrl}/api/subscribers`).then((response) => {
      const { data } = response.body;
      expect(data.results[0].status).to.equal('blocklisted');
      expect(data.results[0].lists.find((s) => s.id === 2).subscription_status).to.equal('unsubscribed');
      expect(data.results[0].lists.find((s) => s.id === 3).subscription_status).to.equal('unsubscribed');
    });
  });

  it('Manages subscription preferences', () => {
    cy.request('GET', `${apiUrl}/api/subscribers`).then((response) => {
      const subUUID = response.body.data.results[1].uuid;

      cy.request('GET', `${apiUrl}/api/campaigns`).then((response) => {
        const campUUID = response.body.data.results[0].uuid;
        cy.loginAndVisit(`${apiUrl}/subscription/${campUUID}/${subUUID}?manage=1`);
        cy.get('a').contains('Manage').click();
      });
    });

    // Change name and unsubscribe from one list.
    cy.get('input[name=name]').clear().type('new-name');
    cy.get('ul.lists input:first').click();
    cy.get('button:first').click();

    cy.request('GET', `${apiUrl}/api/subscribers`).then((response) => {
      const { data } = response.body;
      expect(data.results[1].name).to.equal('new-name');
      expect(data.results[1].lists.find((s) => s.id === 2).subscription_status).to.equal('unsubscribed');
      expect(data.results[1].lists.find((s) => s.id === 3).subscription_status).to.equal('unconfirmed');
    });
  });
});
