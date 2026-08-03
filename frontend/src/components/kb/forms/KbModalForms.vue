<script setup lang="ts">
// KbModalForms mounts every kb/forms/*Form.vue together — each one stays
// closed until useKbModal's shared session targets its own kind (see
// useKbModal.ts's doc comment), so there is exactly one place either page
// needs to render to make every «Изменить»/«Добавить» action work.
import TopicForm from './TopicForm.vue'
import ProductForm from './ProductForm.vue'
import TariffForm from './TariffForm.vue'
import DeliveryZoneForm from './DeliveryZoneForm.vue'
import ContactsForm from './ContactsForm.vue'
import PoliciesForm from './PoliciesForm.vue'
import AssistantFieldForm from './AssistantFieldForm.vue'
</script>

<template>
  <TopicForm />
  <ProductForm />
  <TariffForm />
  <DeliveryZoneForm />
  <ContactsForm />
  <PoliciesForm />
  <AssistantFieldForm />
</template>
