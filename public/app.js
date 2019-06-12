const store = new Vuex.Store({
  state: {
    messages: {}
  },
  mutations: {
    sync (state, messages) {
      state.messages = messages
    },
    add (state, { id, message }) {
      Vue.set(state.messages, id, message)
    },
    del (state, id) {
      Vue.delete(state.messages, id)
    }
  }
})

Vue.component('message', {
  props: {
    id: {
      type: String,
      required: true
    },
    msg: {
      type: Object,
      required: true
    }
  },
  methods: {
    action (data) {
      if (ws !== null) {
        ws.send(JSON.stringify({ ...data }))
      }
    }
  },
  template: `
<tr>
    <td>{{ msg.From }}</td>
    <td>{{ msg.To }}</td>
    <td>{{ msg.Body }}</td>
    <td>
      <!--button @click.prevent="action({action: 'webhook', event: 'sent', id})" type="button">Sent</button>
      <button @click.prevent="action({action: 'webhook', event: 'delivered', id})" type="button">Delivered</button-->
      <button @click.prevent="action({action: 'remove', id})" type="button">Remove</button>
    </td>
</tr>`
})

const app = new Vue({
  store,
  template: `
<table border="1">
    <tr v-if="Object.keys(store.state.messages).length === 0"><td>No messages</td></tr>
    <template v-else>
        <tr><th>From</th><th>To</th><th>Body</th><th>Actions</th></tr>
        <message v-for="(msg, id) in store.state.messages" v-bind:key="id" v-bind:id="id" v-bind:msg="msg"></message>
    </template>
</table>`
})

app.$mount('#app')

/** @type {WebSocket|null} */
let ws = null

function connect () {
  ws = new WebSocket('ws' + window.location.protocol.substring(4) + '//' + window.location.host + '/ws')

  ws.onclose = function (ev) {
    console.log('onclose', ev)
    ws = null
    setTimeout(() => connect(), 5000)
  }

  ws.onmessage = function (ev) {
    /** @type {{action: string, id?: string, data?: Object}} */
    let data = { action: '' }

    try {
      data = JSON.parse(ev.data)
    } catch (err) {
      console.debug(err)
      return
    }

    switch (data.action) {
      case 'sync':
        store.commit('sync', data.data)
        break
      case 'add':
        store.commit('add', { id: data.id, message: data.data })
        break
      case 'del':
        store.commit('del', data.id)
        break
      default:
        console.debug('unknown action', data)
    }
  }
}

connect()
