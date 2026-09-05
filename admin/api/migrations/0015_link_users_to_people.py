from django.conf import settings
from django.db import migrations

# Prod accounts and the person each one is. Spelled out rather than derived from
# the username: 'Elya' is not a prefix of 'Ellina Metzger', so matching on the
# first word of the name would link Jesse and Helen and quietly skip her.
USERNAME_TO_PERSON = {
    'Jesse': 'Jesse Metzger',
    'Elya': 'Ellina Metzger',
    'Helen': 'Helen Metzger',
}


def link(apps, schema_editor):
    """Point prod's existing people at the accounts they belong to.

    Keyed off usernames rather than user ids, like the owners backfill before
    it, so this is a no-op anywhere those accounts do not exist -- local dev has
    admin, Boris and Pavel, whose links the fixtures carry instead.

    A person already linked to some other account is left alone.
    """
    Person = apps.get_model('api', 'Person')
    User = apps.get_model(settings.AUTH_USER_MODEL)

    users = {u.username: u.pk for u in User.objects.filter(username__in=USERNAME_TO_PERSON)}

    for username, name in USERNAME_TO_PERSON.items():
        user_id = users.get(username)
        if user_id is None:
            continue
        Person.objects.filter(name=name, user__isnull=True).update(user_id=user_id)


def unlink(apps, schema_editor):
    Person = apps.get_model('api', 'Person')
    User = apps.get_model(settings.AUTH_USER_MODEL)

    user_ids = User.objects.filter(username__in=USERNAME_TO_PERSON).values_list('pk', flat=True)

    Person.objects.filter(user_id__in=user_ids).update(user=None)


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0014_person_user'),
    ]

    operations = [
        migrations.RunPython(link, unlink),
    ]
